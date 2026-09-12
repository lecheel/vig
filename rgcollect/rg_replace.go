package rgcollect

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

// ──────────────────────────────────────────────────────────────────
//  Replace flow for the [rg] grouped buffer (F11 view)
//
//  Two sub-modes share one buffer:
//
//    browse  — the default [rg] key handler: navigate, open, exclude,
//              recall, undo the last apply.
//    replace — Tab from browse pushes this sub-mode: type text, move
//              the insertion point, toggle preview, apply, cancel.
//
//  The sub-mode is not a wig mode on the stack; instead we swap the
//  buffer's key handler.  This matches how the rest of the editor
//  swaps handlers per popup/widget and keeps the replace flow entirely
//  local to this package.
//
//  Key bindings (browse):
//    Enter    open file at cursor
//    Tab      enter replace sub-mode
//    Space    toggle inclusion of cursor's row (or whole file on header)
//    u        undo the last apply (file-level)
//    l / L    jump to next / previous file header
//    q        close [rg] view
//
//  Key bindings (replace):
//    <rune>   insert at replace cursor
//    Space    insert space
//    Bksp     delete rune before cursor
//    Del      delete rune under cursor
//    Left/Rt  move replace cursor
//    Home/End jump to start / end of replacement
//    Enter    apply to every included match, pop back to browse
//    Esc      discard the replacement, pop back to browse
//
//  The match span is highlighted the moment replace mode is entered.
//  As soon as any replacement text is typed, the preview goes live
//  automatically: the old match renders struck through immediately
//  followed by the new text, e.g. old(NEW). No separate toggle.
// ──────────────────────────────────────────────────────────────────

type rgPhase int

const (
	rgPhaseBrowse rgPhase = iota
	rgPhaseReplace
	rgPhaseSearch
)

// appliedFile captures the on-disk bytes of a file just before the
// replace apply overwrote them, so `u` in browse can restore it.
// This is a file-level undo, distinct from the buffer's own undo
// history (see rgview.md §9).
type appliedFile struct {
	path     string
	original []byte
}

// rgReplaceState holds all mutable replace-flow state.  It lives at
// package scope so the highlighter can read it during rendering; all
// mutations happen on the editor's main goroutine.
var rgReplaceState = struct {
	phase         rgPhase
	replacement   []rune
	replaceCursor int
	searchQuery   []rune
	searchCursor  int
	applied       []appliedFile
	statusMsg     string
}{
	phase: rgPhaseBrowse,
}

// rgRSP is a convenience alias so rgcollect.go can reference the state
// without the longer path.
var rgRSP = &rgReplaceState

// rgTypeable enumerates the printable runes captured by the replace
// sub-mode's input handler.
const rgTypeable = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,;:/\\-_=+[]{}()<>!@#$%^&*|~`'\""

// ── Browse sub-mode ──────────────────────────────────────────────

// rgInstallBrowseHandler restores the browse key handler on buf.
// Called on view open and whenever the replace sub-mode exits.
func rgInstallBrowseHandler(buf *wig.Buffer) {
	// Use NewKeyHandler (not DefaultKeyHandler) so default editor normal mode
	// keys cannot leak through and cause edits or unexpected behavior in the
	// search results buffer.
	buf.KeyHandler = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Enter":  CmdRgEnter,
			"/":      rgEnterSearch,
			"s":      rgEnterSearch,
			"Tab":    rgEnterReplace,
			"Space":  rgToggleExclude,
			"u":      rgUndoApply,
			"l":      rgJumpNextFile,
			"L":      rgJumpPrevFile,
			"q":      rgClose,
			"Esc":    rgClose,
			"j":      rgCursorDown,
			"Down":   rgCursorDown,
			"k":      rgCursorUp,
			"Up":     rgCursorUp,
			"ctrl+d": rgPageDown,
			"PgDn":   rgPageDown,
			"ctrl+u": rgPageUp,
			"PgUp":   rgPageUp,
			"g":      rgGoTop,
			"Home":   rgGoTop,
			"G":      rgGoBottom,
			"End":    rgGoBottom,
			"h":      rgCursorLeftBrowse,
			"Left":   rgCursorLeftBrowse,
			"Right":  rgCursorRightBrowse,
		},
	})

	// Block every unbound key so it can never leak to the buffer or editor.
	buf.KeyHandler.Fallback(func(ctx wig.Context, ev *tcell.EventKey) {
		rgRenderBrowseStatus(ctx)
	})
}

// rgEnterReplace pushes the replace sub-mode: swap the key handler,
// reset the cursor to the end of the current replacement, and clear
// any stale status message.
func rgEnterReplace(ctx wig.Context) {
	rgRSP.phase = rgPhaseReplace
	rgRSP.replaceCursor = len(rgRSP.replacement)
	rgRSP.statusMsg = ""
	rgInstallReplaceHandler(ctx.Buf)
	// Force a highlighter rebuild now so the match span switches to the
	// replace-mode (orange) style immediately, even before any text is
	// typed. Redraw() alone repaints from cached spans; the reload inside
	// rgRefreshPreview is what actually invalidates them.
	rgRefreshPreview(ctx)
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

// rgToggleExclude flips the inclusion flag on the cursor's row.  On a
// match row only that match flips; on a file header all matches in
// that file flip together, with the direction decided by "was any of
// them included".  Excluded rows render dimmed and are skipped by the
// replace apply.
func rgToggleExclude(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur == nil {
		return
	}
	hl, ok := ctx.Buf.Highlighter.(*RgHighlighter)
	if !ok {
		return
	}
	entry, ok := hl.LineMap[cur.Line]
	if !ok {
		return
	}

	switch entry.kind {
	case 2:
		if entry.resultIdx >= 0 && entry.resultIdx < len(rgState.results) {
			rgState.results[entry.resultIdx].Excluded = !rgState.results[entry.resultIdx].Excluded
		}
	case 1:
		// Compute whether any result in this file is currently included;
		// if so, exclude them all, otherwise include them all.
		anyIncluded := false
		idxs := []int{}
		for i, r := range rgState.results {
			if r.FilePath != entry.filePath {
				continue
			}
			idxs = append(idxs, i)
			if !r.Excluded {
				anyIncluded = true
			}
		}
		exclude := anyIncluded
		for _, i := range idxs {
			rgState.results[i].Excluded = exclude
		}
	}
	// If preview is on, an excluded result must drop its inline splice
	// and revert to the original (dimmed) text; if the toggle just
	// included a result back, its splice must reappear. Refresh handles
	// both.
	rgRefreshPreview(ctx)
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

// rgUndoApply restores every file recorded by the last apply from the
// bytes captured before the write, then clears the applied slice.
// Silently no-ops when there is nothing to undo.
func rgUndoApply(ctx wig.Context) {
	applied := rgRSP.applied
	if len(applied) == 0 {
		rgRSP.statusMsg = "Nothing to undo"
		rgRenderBrowseStatus(ctx)
		return
	}
	rgRSP.applied = nil

	restored := 0
	for _, a := range applied {
		if err := os.WriteFile(a.path, a.original, 0644); err == nil {
			restored++
		}
	}
	rgRSP.statusMsg = fmt.Sprintf("Undo: restored %d file(s)", restored)
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

// ── Replace sub-mode ─────────────────────────────────────────────

// rgInstallReplaceHandler swaps in the raw input handler for the
// replace sub-mode.  Every printable key becomes a text insertion at
// the replace cursor; navigation and control keys are wired explicitly.
func rgInstallReplaceHandler(buf *wig.Buffer) {
	km := wig.KeyMap{}
	for _, r := range rgTypeable {
		c := r
		km[string(c)] = func(ctx wig.Context) { rgInsertRune(ctx, c) }
	}
	km["Space"] = func(ctx wig.Context) { rgInsertRune(ctx, ' ') }
	km["Backspace"] = rgBackspace
	km["Backspace2"] = rgBackspace
	km["ctrl+h"] = rgBackspace
	km["BS"] = rgBackspace
	km["Delete"] = rgDeleteForward
	km["Left"] = rgCursorLeft
	km["Right"] = rgCursorRight
	km["Home"] = rgCursorHome
	km["End"] = rgCursorEnd
	km["Enter"] = rgApplyReplace
	km["Esc"] = rgCancelReplace
	km["ctrl+c"] = rgCancelReplace

	// NewKeyHandler (not DefaultKeyHandler) so unbound keys do not
	// fall through to the editor and cause surprise edits.
	buf.KeyHandler = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: km,
	})
}

func rgInsertRune(ctx wig.Context, r rune) {
	i := rgRSP.replaceCursor
	if i < 0 || i > len(rgRSP.replacement) {
		i = len(rgRSP.replacement)
	}
	newRepl := make([]rune, 0, len(rgRSP.replacement)+1)
	newRepl = append(newRepl, rgRSP.replacement[:i]...)
	newRepl = append(newRepl, r)
	newRepl = append(newRepl, rgRSP.replacement[i:]...)
	rgRSP.replacement = newRepl
	rgRSP.replaceCursor = i + 1
	rgRefreshPreview(ctx)
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgBackspace(ctx wig.Context) {
	i := rgRSP.replaceCursor
	if i <= 0 {
		// Backspace on an already-empty prompt dismisses it.
		rgCancelReplace(ctx)
		return
	}
	rgRSP.replacement = append(rgRSP.replacement[:i-1], rgRSP.replacement[i:]...)
	rgRSP.replaceCursor = i - 1
	rgRefreshPreview(ctx)
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgDeleteForward(ctx wig.Context) {
	i := rgRSP.replaceCursor
	if i >= len(rgRSP.replacement) {
		return
	}
	rgRSP.replacement = append(rgRSP.replacement[:i], rgRSP.replacement[i+1:]...)
	rgRefreshPreview(ctx)
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorLeft(ctx wig.Context) {
	if rgRSP.replaceCursor > 0 {
		rgRSP.replaceCursor--
	}
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorRight(ctx wig.Context) {
	if rgRSP.replaceCursor < len(rgRSP.replacement) {
		rgRSP.replaceCursor++
	}
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorHome(ctx wig.Context) {
	rgRSP.replaceCursor = 0
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorEnd(ctx wig.Context) {
	rgRSP.replaceCursor = len(rgRSP.replacement)
	rgRenderReplaceStatus(ctx)
	ctx.Editor.Redraw()
}

// rgCancelReplace discards the replacement text and pops back to
// browse.  The result list and any applied undo history are untouched.
// The buffer content is restored to the pristine search-result text
// (the inline preview splice is undone).
func rgCancelReplace(ctx wig.Context) {
	rgRSP.phase = rgPhaseBrowse
	rgRSP.replacement = nil
	rgRSP.replaceCursor = 0
	rgRSP.statusMsg = ""
	rgInstallBrowseHandler(ctx.Buf)
	rgRefreshPreview(ctx)
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

const rgBrowseHint = "[Enter] Open  [/] Search  [Tab] Replace  [SPC] Exclude  [l/L] File  [u] Undo  [q] Close"

// rgRenderBrowseStatus writes the current status message to the popup's
// echo row. The shortcut hint is deliberately NOT appended here — it is
// rendered exactly once by the popup's own hint row (RgViewWidget.Render,
// hintRow). When there is no status message, e.Message is cleared directly
// rather than through EchoMessage(""), which would append a blank entry to
// the [Messages] buffer on every cursor move.
func rgRenderBrowseStatus(ctx wig.Context) {
	if rgRSP.statusMsg != "" {
		ctx.Editor.EchoMessage(rgRSP.statusMsg)
		return
	}
	ctx.Editor.Message = ""
}

func rgClampCursor(buf *wig.Buffer, cur *wig.Cursor) {
	if buf == nil || cur == nil {
		return
	}
	if cur.Line < 0 {
		cur.Line = 0
	}
	if cur.Line >= buf.Lines.Len {
		cur.Line = max(0, buf.Lines.Len-1)
	}
	line := wig.CursorLineByNum(buf, cur.Line)
	if line != nil {
		lineLen := len([]rune(line.Value.String()))
		if cur.Char > lineLen {
			cur.Char = lineLen
		}
		if cur.Char < 0 {
			cur.Char = 0
		}
	}
}

func rgCursorDown(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && ctx.Buf != nil && cur.Line < ctx.Buf.Lines.Len-1 {
		cur.Line++
		rgClampCursor(ctx.Buf, cur)
		updateCurrentPos(cur.Line)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorUp(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && ctx.Buf != nil && cur.Line > 0 {
		cur.Line--
		rgClampCursor(ctx.Buf, cur)
		updateCurrentPos(cur.Line)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgPageDown(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && ctx.Buf != nil {
		cur.Line += 15
		rgClampCursor(ctx.Buf, cur)
		updateCurrentPos(cur.Line)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgPageUp(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && ctx.Buf != nil {
		cur.Line -= 15
		rgClampCursor(ctx.Buf, cur)
		updateCurrentPos(cur.Line)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgGoTop(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil {
		cur.Line = 0
		cur.Char = 0
		updateCurrentPos(0)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgGoBottom(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && ctx.Buf != nil && ctx.Buf.Lines.Len > 0 {
		cur.Line = ctx.Buf.Lines.Len - 1
		cur.Char = 0
		updateCurrentPos(cur.Line)
	}
	rgRSP.statusMsg = ""
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorLeftBrowse(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil && cur.Char > 0 {
		cur.Char--
	}
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCursorRightBrowse(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	if cur != nil {
		cur.Char++
	}
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

// rgClose closes the [rg] buffer view and returns to the previous buffer.
func rgClose(ctx wig.Context) {
	rgMutex.Lock()
	defer rgMutex.Unlock()

	cur := wig.ContextCursorGet(ctx)
	if cur != nil {
		if entry, ok := rgState.lineMap[cur.Line]; ok && entry.kind == 2 {
			saveLastPosition(entry.resultIdx, cur.Line)
		} else {
			saveLastPosition(-1, cur.Line)
		}
	}

	rgRSP.phase = rgPhaseBrowse
	rgRSP.replacement = nil
	rgRSP.replaceCursor = 0
	rgRSP.statusMsg = ""

	targetBuf := rgPreviousBuf
	bufValid := false
	if targetBuf != nil {
		for _, b := range ctx.Editor.Buffers {
			if b == targetBuf {
				bufValid = true
				break
			}
		}
	}

	if !bufValid {
		targetBuf = nil
		for i := len(ctx.Editor.Buffers) - 1; i >= 0; i-- {
			b := ctx.Editor.Buffers[i]
			if b.FilePath != "[rg]" && !strings.HasPrefix(b.FilePath, "[rgcollect ") {
				targetBuf = b
				break
			}
		}
	}

	if targetBuf == nil {
		targetBuf = wig.NewBuffer()
		ctx.Editor.Buffers = append(ctx.Editor.Buffers, targetBuf)
	}

	ctx.Buf = targetBuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx)

	// The widget early-returns when the active buffer is not [rg], so
	// leaving it on the UI stack would be harmless, but popping it here
	// keeps the stack tidy across many F11 opens.
	CloseRgViewWidget(ctx.Editor)

	ctx.Editor.EchoMessage("")
	ctx.Editor.Redraw()
}

// ── Search sub-mode (input prompt with BS support) ───────────────

func rgEnterSearch(ctx wig.Context) {
	rgRSP.phase = rgPhaseSearch
	rgRSP.searchQuery = nil
	rgRSP.searchCursor = 0
	rgRSP.statusMsg = ""
	rgInstallSearchHandler(ctx.Buf)
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgCancelSearch(ctx wig.Context) {
	rgRSP.phase = rgPhaseBrowse
	rgRSP.searchQuery = nil
	rgRSP.searchCursor = 0
	rgInstallBrowseHandler(ctx.Buf)
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

func rgInstallSearchHandler(buf *wig.Buffer) {
	km := wig.KeyMap{}
	for _, r := range rgTypeable {
		c := r
		km[string(c)] = func(ctx wig.Context) { rgInsertSearchRune(ctx, c) }
	}
	km["Space"] = func(ctx wig.Context) { rgInsertSearchRune(ctx, ' ') }
	km["Backspace"] = rgSearchBackspace
	km["Backspace2"] = rgSearchBackspace
	km["ctrl+h"] = rgSearchBackspace
	km["BS"] = rgSearchBackspace
	km["Delete"] = rgSearchDeleteForward
	km["Left"] = rgSearchCursorLeft
	km["Right"] = rgSearchCursorRight
	km["Home"] = rgSearchCursorHome
	km["End"] = rgSearchCursorEnd
	km["Enter"] = rgApplySearch
	km["Esc"] = rgCancelSearch
	km["ctrl+c"] = rgCancelSearch

	buf.KeyHandler = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: km,
	})
}

func rgInsertSearchRune(ctx wig.Context, r rune) {
	i := rgRSP.searchCursor
	if i < 0 || i > len(rgRSP.searchQuery) {
		i = len(rgRSP.searchQuery)
	}
	newQuery := make([]rune, 0, len(rgRSP.searchQuery)+1)
	newQuery = append(newQuery, rgRSP.searchQuery[:i]...)
	newQuery = append(newQuery, r)
	newQuery = append(newQuery, rgRSP.searchQuery[i:]...)
	rgRSP.searchQuery = newQuery
	rgRSP.searchCursor = i + 1
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchBackspace(ctx wig.Context) {
	i := rgRSP.searchCursor
	if i <= 0 {
		// Backspace on an already-empty prompt dismisses it.
		rgCancelSearch(ctx)
		return
	}
	rgRSP.searchQuery = append(rgRSP.searchQuery[:i-1], rgRSP.searchQuery[i:]...)
	rgRSP.searchCursor = i - 1
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchDeleteForward(ctx wig.Context) {
	i := rgRSP.searchCursor
	if i >= len(rgRSP.searchQuery) {
		return
	}
	rgRSP.searchQuery = append(rgRSP.searchQuery[:i], rgRSP.searchQuery[i+1:]...)
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchCursorLeft(ctx wig.Context) {
	if rgRSP.searchCursor > 0 {
		rgRSP.searchCursor--
	}
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchCursorRight(ctx wig.Context) {
	if rgRSP.searchCursor < len(rgRSP.searchQuery) {
		rgRSP.searchCursor++
	}
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchCursorHome(ctx wig.Context) {
	rgRSP.searchCursor = 0
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgSearchCursorEnd(ctx wig.Context) {
	rgRSP.searchCursor = len(rgRSP.searchQuery)
	rgRenderSearchStatus(ctx)
	ctx.Editor.Redraw()
}

func rgRenderSearchStatus(ctx wig.Context) {
	before := string(rgRSP.searchQuery[:rgRSP.searchCursor])
	after := string(rgRSP.searchQuery[rgRSP.searchCursor:])
	ctx.Editor.EchoMessage(fmt.Sprintf(
		"─── [SEARCH] ─── query: ▐%s█%s▌ ─── Enter: search  Esc: cancel",
		before, after,
	))
}

func rgApplySearch(ctx wig.Context) {
	query := strings.TrimSpace(string(rgRSP.searchQuery))
	if query == "" {
		ctx.Editor.EchoMessage("Empty search query")
		rgCancelSearch(ctx)
		return
	}

	ctx.Editor.EchoMessage("Searching for '" + query + "'...")
	ctx.Editor.Redraw()

	rootDir := ctx.Editor.Projects.GetRoot()
	if rootDir == "" {
		rootDir = "."
	}

	cmd := exec.Command("rg", "--vimgrep", query)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		rgRSP.phase = rgPhaseBrowse
		rgRSP.searchQuery = nil
		rgRSP.searchCursor = 0
		rgRSP.statusMsg = "No matches for '" + query + "'"
		rgInstallBrowseHandler(ctx.Buf)
		rgRenderBrowseStatus(ctx)
		ctx.Editor.Redraw()
		return
	}

	lines := strings.Split(string(out), "\n")
	var locs []wig.Location
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, ":", 4)
		if len(parts) < 4 {
			continue
		}
		var lineNum, colNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)
		fmt.Sscanf(parts[2], "%d", &colNum)
		charIdx := colNum - 1
		if charIdx < 0 {
			charIdx = 0
		}
		// rg --vimgrep reports a 1-based *byte* column (for Vim quickfix
		// compatibility), so the same conversion as parseRgJSON in
		// commands/helpers.go is required: turn the byte offset into a
		// rune offset before it reaches wig.Location.Char, or the
		// highlight shifts right on any line whose prefix contains
		// multi-byte runes (box-drawing `─` in comments, Unicode
		// identifiers, emoji, ...). On pure-ASCII lines byte and rune
		// offsets coincide, so this is a no-op there.
		lineText := parts[3]
		if charIdx > len(lineText) {
			charIdx = len(lineText)
		}
		charIdx = utf8.RuneCountInString(lineText[:charIdx])
		locs = append(locs, wig.Location{
			FilePath: parts[0],
			Line:     lineNum,
			Char:     charIdx,
			Text:     lineText,
		})
	}

	if len(locs) == 0 {
		rgRSP.phase = rgPhaseBrowse
		rgRSP.searchQuery = nil
		rgRSP.searchCursor = 0
		rgRSP.statusMsg = "No matches for '" + query + "'"
		rgInstallBrowseHandler(ctx.Buf)
		rgRenderBrowseStatus(ctx)
		ctx.Editor.Redraw()
		return
	}

	rgRSP.phase = rgPhaseBrowse
	rgRSP.searchQuery = nil
	rgRSP.searchCursor = 0
	rgRSP.statusMsg = ""

	InitGrouped(ctx, query, locs)
}

// rgRenderReplaceStatus is a no-op kept for call-site compatibility. The
// replace prompt — including the insertion-point block — is rendered by
// the popup header (RgViewWidget.Render / rgReplaceLine), so echoing it
// here would show it twice. Any stale echo from before replace mode was
// entered is cleared by setting e.Message directly.
func rgRenderReplaceStatus(ctx wig.Context) {
	ctx.Editor.Message = ""
}

func splicePreviewText(text string, matches []MatchSpan, replacement string) string {
	runes := []rune(text)
	var sb strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m.Start, m.End
		if start < 0 {
			start = 0
		}
		if start < last {
			start = last
		}
		if start > len(runes) {
			start = len(runes)
		}
		if end < start {
			end = start
		}
		if end > len(runes) {
			end = len(runes)
		}
		sb.WriteString(string(runes[last:start]))
		sb.WriteString(string(runes[start:end]))
		sb.WriteString(replacement)
		last = end
	}
	if last < len(runes) {
		sb.WriteString(string(runes[last:]))
	}
	return sb.String()
}

// rgRefreshPreview rebuilds the [rg] buffer content so that, once a
// non-empty replacement has been typed, each included match is
// rendered inline as:
//
//	<prefix><before><original match><replacement><after>
//
// This kicks in automatically as soon as there is replacement text —
// no separate preview toggle. The original match is kept in the
// buffer text (not deleted) so the highlighter can draw it
// struck-through; the replacement is spliced in immediately after it.
// When the replacement is empty, or a result is excluded, the
// original result text is restored.
func rgRefreshPreview(ctx wig.Context) {
	buf := ctx.Buf
	if buf == nil {
		return
	}
	hl, ok := buf.Highlighter.(*RgHighlighter)
	if !ok {
		return
	}

	showPreview := rgRSP.phase == rgPhaseReplace &&
		len(rgRSP.replacement) > 0
	replacement := string(rgRSP.replacement)

	cur := wig.ContextCursorGet(ctx)
	savedLine, savedChar := 0, 0
	if cur != nil {
		savedLine, savedChar = cur.Line, cur.Char
	}

	parts := make([]string, 0, buf.Lines.Len)
	for i := 0; i < buf.Lines.Len; i++ {
		line := wig.CursorLineByNum(buf, i)
		if line == nil {
			break
		}
		content := strings.TrimSuffix(string(line.Value), "\n")

		if entry, ok := hl.LineMap[i]; ok && entry.kind == 2 {
			if entry.resultIdx >= 0 && entry.resultIdx < len(rgState.results) {
				result := rgState.results[entry.resultIdx]
				if showPreview && !result.Excluded {
					content = fmt.Sprintf(
						"%d:\t%s",
						result.Line,
						splicePreviewText(result.Text, result.GetMatches(), replacement),
					)
				} else {
					content = fmt.Sprintf("%d:\t%s", result.Line, result.Text)
				}
			}
		}
		parts = append(parts, content)
	}

	wig.ReloadBufferContent(ctx, strings.Join(parts, "\n"))

	if cur != nil {
		if savedLine >= 0 && savedLine < buf.Lines.Len {
			cur.Line = savedLine
			cur.Char = savedChar
		}
	}
	ctx.Editor.Redraw()
}

// ── Apply ────────────────────────────────────────────────────────

// rgApplyReplace writes the replacement text over every included match
// span, records each file's original bytes for one-level undo, and
// returns to browse.  Edits within a file are applied bottom-up /
// right-to-left so no splice shifts the coordinates of another.
func rgApplyReplace(ctx wig.Context) {
	replacement := string(rgRSP.replacement)
	if replacement == "" {
		ctx.Editor.EchoMessage("Empty replacement — nothing to apply")
		return
	}

	// Group included matches by file.
	byFile := map[string][]RgResult{}
	for _, r := range rgState.results {
		if r.Excluded {
			continue
		}
		byFile[r.FilePath] = append(byFile[r.FilePath], r)
	}
	if len(byFile) == 0 {
		ctx.Editor.EchoMessage("No included matches to replace")
		return
	}

	// Deterministic file order so status messages are reproducible.
	paths := make([]string, 0, len(byFile))
	for p := range byFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	totalReplacements := 0
	filesChanged := 0
	var applied []appliedFile

	for _, path := range paths {
		matches := byFile[path]

		original, err := os.ReadFile(path)
		if err != nil {
			ctx.Editor.EchoMessage("Read error: " + path + ": " + err.Error())
			break
		}

		newContent, n := rgSpliceFile(string(original), matches, replacement)
		if n == 0 {
			continue
		}
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			ctx.Editor.EchoMessage("Write error: " + path + ": " + err.Error())
			break
		}

		applied = append(applied, appliedFile{path: path, original: original})
		totalReplacements += n
		filesChanged++
	}

	// Record undo state and reset the replace sub-mode.
	if len(applied) > 0 {
		rgRSP.applied = applied
	}
	rgRSP.phase = rgPhaseBrowse
	rgRSP.replacement = nil
	rgRSP.replaceCursor = 0

	rgInstallBrowseHandler(ctx.Buf)

	// Reload any buffers that are showing the files we just touched, so
	// the editor's view matches disk.
	for _, a := range applied {
		for _, b := range ctx.Editor.Buffers {
			if b.FilePath == a.path && !b.Dirty {
				_ = wig.BufferReloadFile(b)
				if b.Highlighter != nil {
					b.Highlighter.Build()
				}
				ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: b})
			}
		}
	}

	if totalReplacements == 0 {
		rgRSP.statusMsg = "No replacements applied"
	} else {
		rgRSP.statusMsg = fmt.Sprintf("Applied %d replacement(s) in %d file(s) — press u to undo", totalReplacements, filesChanged)
	}
	rgRenderBrowseStatus(ctx)
	ctx.Editor.Redraw()
}

// rgSpliceFile applies replacement to every match in content. Each
// match carries rune offsets into its line; sorting descending by
// (line, start) means a later splice never shifts an earlier match's
// coordinates within the same line.
func rgSpliceFile(content string, matches []RgResult, replacement string) (string, int) {
	lines := strings.Split(content, "\n")
	type singleMatch struct {
		line  int
		start int
		end   int
	}
	var allMatches []singleMatch
	for _, r := range matches {
		for _, m := range r.GetMatches() {
			allMatches = append(allMatches, singleMatch{
				line:  r.Line,
				start: m.Start,
				end:   m.End,
			})
		}
	}

	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].line != allMatches[j].line {
			return allMatches[i].line > allMatches[j].line
		}
		return allMatches[i].start > allMatches[j].start
	})

	applied := 0
	for _, m := range allMatches {
		idx := m.line - 1
		if idx < 0 || idx >= len(lines) {
			continue
		}
		runes := []rune(lines[idx])
		start, end := m.start, m.end
		if start < 0 || start > len(runes) {
			continue
		}
		if end < start || end > len(runes) {
			end = len(runes)
		}
		lines[idx] = string(runes[:start]) + replacement + string(runes[end:])
		applied++
	}
	return strings.Join(lines, "\n"), applied
}

// ── Navigation helpers ───────────────────────────────────────────

func rgJumpNextFile(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	hl, ok := ctx.Buf.Highlighter.(*RgHighlighter)
	if !ok {
		return
	}
	for i := cur.Line + 1; i < ctx.Buf.Lines.Len; i++ {
		if entry, ok := hl.LineMap[i]; ok && entry.kind == 1 {
			cur.Line = i
			cur.Char = 0
			updateCurrentPos(i)
			wig.CmdCursorCenter(ctx)
			rgRenderBrowseStatus(ctx)
			return
		}
	}
}

func rgJumpPrevFile(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	hl, ok := ctx.Buf.Highlighter.(*RgHighlighter)
	if !ok {
		return
	}
	for i := cur.Line - 1; i >= 0; i-- {
		if entry, ok := hl.LineMap[i]; ok && entry.kind == 1 {
			cur.Line = i
			cur.Char = 0
			updateCurrentPos(i)
			wig.CmdCursorCenter(ctx)
			rgRenderBrowseStatus(ctx)
			return
		}
	}
}
