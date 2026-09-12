package rgcollect

import (
	"fmt"
	"strings"
	"sync"

	"github.com/firstrow/wig"
)

// ──────────────────────────────────────────────────────────────────
//  Search & Replace for the [rg] grouped buffer (F11 view)
//
//  Flow:
//    Tab (or /)  -> enter search mode, start typing the search pattern
//    Tab         -> toggle between search <-> replace input
//    Enter       -> in search mode: advance to replace input
//                   in replace mode: apply replacement to every match
//    Backspace   -> delete last typed character
//    Esc         -> cancel the session and restore normal keybindings
// ──────────────────────────────────────────────────────────────────

type rgReplacePhase int

const (
	rgPhaseInactive rgReplacePhase = iota
	rgPhaseSearch
	rgPhaseReplace
)

type rgReplaceStateT struct {
	phase       rgReplacePhase
	searchQuery string
	replaceText string
}

var rgRS = rgReplaceStateT{}
var rgMu sync.Mutex

// rgTypeable enumerates the printable characters captured by the input handler.
const rgTypeable = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,;:/\\-_=+[]{}()<>!@#$%^&*|~`'\""

// RgIsActive reports whether a search/replace session is in progress.
func RgIsActive() bool {
	rgMu.Lock()
	defer rgMu.Unlock()
	return rgRS.phase != rgPhaseInactive
}

// rgDefaultKeyMap returns the [rg] buffer's normal-mode key map.  It contains
// the standard navigation bindings plus the entry points for the search/replace
// flow (Tab and /).
//
// This is a function rather than a package-level variable to avoid an
// initialization cycle: the map references CmdRgReplaceStart, which (through
// rgInstallInputHandler -> rgEnter -> rgCancel -> rgInstallDefaultHandler)
// refers back to the map.  Function bodies are only evaluated when called,
// which breaks the cycle at init time.
func rgDefaultKeyMap() wig.ModeKeyMap {
	return wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Enter": CmdRgEnter,
			"Tab":   CmdRgReplaceStart,
			"/":     CmdRgReplaceStart,
			"Esc":   rgCancelIfActive,
			"l":     rgJumpNextFile,
			"L":     rgJumpPrevFile,
		},
	}
}

// rgInstallDefaultHandler sets the standard [rg] buffer key handler.
func rgInstallDefaultHandler(buf *wig.Buffer) {
	buf.KeyHandler = wig.DefaultKeyHandler(rgDefaultKeyMap())
}

// ── Entry point ──────────────────────────────────────────────────

// CmdRgReplaceStart initiates the search/replace flow from the [rg] buffer.
// The first invocation enters search mode; Tab inside the input handler
// toggles between search and replace modes.
func CmdRgReplaceStart(ctx wig.Context) {
	rgMu.Lock()
	if rgRS.phase == rgPhaseInactive {
		rgRS = rgReplaceStateT{phase: rgPhaseSearch}
	}
	rgMu.Unlock()
	rgInstallInputHandler(ctx.Buf)
	rgRenderPrompt(ctx)
}

// rgCancelIfActive cancels the search/replace session if one is in progress.
func rgCancelIfActive(ctx wig.Context) {
	if RgIsActive() {
		rgCancel(ctx)
	}
}

// ── Input handler ────────────────────────────────────────────────

// rgInstallInputHandler swaps the [rg] buffer's key handler for one that
// captures raw printable keys for the search/replace prompt.
func rgInstallInputHandler(buf *wig.Buffer) {
	km := wig.KeyMap{}
	for _, r := range rgTypeable {
		c := r
		km[string(c)] = func(ctx wig.Context) { rgAppendChar(ctx, c) }
	}
	km["Space"] = func(ctx wig.Context) { rgAppendChar(ctx, ' ') }
	km["Backspace"] = rgBackspace
	km["Tab"] = rgTab
	km["Enter"] = rgEnter
	km["Esc"] = rgCancel
	km["ctrl+c"] = rgCancel

	buf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: km,
	})
}

// ── Key handlers ─────────────────────────────────────────────────

func rgAppendChar(ctx wig.Context, c rune) {
	rgMu.Lock()
	if rgRS.phase == rgPhaseSearch {
		rgRS.searchQuery += string(c)
	} else if rgRS.phase == rgPhaseReplace {
		rgRS.replaceText += string(c)
	}
	rgMu.Unlock()
	rgRenderPrompt(ctx)
}

func rgBackspace(ctx wig.Context) {
	rgMu.Lock()
	if rgRS.phase == rgPhaseSearch && len(rgRS.searchQuery) > 0 {
		rgRS.searchQuery = rgRS.searchQuery[:len(rgRS.searchQuery)-1]
	} else if rgRS.phase == rgPhaseReplace && len(rgRS.replaceText) > 0 {
		rgRS.replaceText = rgRS.replaceText[:len(rgRS.replaceText)-1]
	}
	rgMu.Unlock()
	rgRenderPrompt(ctx)
}

func rgTab(ctx wig.Context) {
	rgMu.Lock()
	switch rgRS.phase {
	case rgPhaseSearch:
		rgRS.phase = rgPhaseReplace
	case rgPhaseReplace:
		rgRS.phase = rgPhaseSearch
	}
	rgMu.Unlock()
	rgRenderPrompt(ctx)
}

func rgEnter(ctx wig.Context) {
	rgMu.Lock()
	phase := rgRS.phase
	query := rgRS.searchQuery
	replacement := rgRS.replaceText
	rgMu.Unlock()

	if phase == rgPhaseSearch {
		// Enter from search mode: switch to replace mode (if there is a
		// search pattern) or exit if the query is empty.
		if query == "" {
			rgCancel(ctx)
			return
		}
		rgMu.Lock()
		rgRS.phase = rgPhaseReplace
		rgMu.Unlock()
		rgRenderPrompt(ctx)
		return
	}

	// Replace mode: apply the replacement to all matches.
	if query == "" {
		ctx.Editor.EchoMessage("no search query")
		return
	}
	rgApplyReplace(ctx, query, replacement)
}

func rgCancel(ctx wig.Context) {
	rgMu.Lock()
	rgRS = rgReplaceStateT{}
	rgMu.Unlock()
	ctx.Editor.EchoMessage("")
	rgInstallDefaultHandler(ctx.Buf)
	ctx.Editor.Redraw()
}

// ── Prompt rendering ─────────────────────────────────────────────

func rgRenderPrompt(ctx wig.Context) {
	rgMu.Lock()
	defer rgMu.Unlock()

	var msg string
	switch rgRS.phase {
	case rgPhaseSearch:
		msg = fmt.Sprintf("Search: %s   [Tab: Replace  Enter: Apply  Esc: Cancel]", rgRS.searchQuery)
	case rgPhaseReplace:
		msg = fmt.Sprintf("Replace: %s   [Tab: Search  Enter: Apply  Esc: Cancel]", rgRS.replaceText)
	default:
		return
	}
	ctx.Editor.EchoMessage(msg)
	ctx.Editor.Redraw()
}

// ── Apply ────────────────────────────────────────────────────────

func rgApplyReplace(ctx wig.Context, query, replacement string) {
	rgMu.Lock()
	results := append([]RgResult(nil), rgState.results...)
	rgMu.Unlock()

	// Group matching result line numbers by file.
	byFile := map[string][]int{}
	for _, r := range results {
		if !strings.Contains(r.Text, query) {
			continue
		}
		byFile[r.FilePath] = append(byFile[r.FilePath], r.Line-1)
	}

	totalLines := 0
	filesChanged := 0

	for path, lines := range byFile {
		buf, err := ctx.Editor.OpenFile(path)
		if err != nil {
			ctx.Editor.LogError(err)
			continue
		}

		// Build the modified content line by line.  Replacement is
		// restricted to the specific line numbers that rg reported as
		// matches, so we never touch unrelated occurrences elsewhere
		// in the same file.
		matchSet := make(map[int]bool, len(lines))
		for _, ln := range lines {
			matchSet[ln] = true
		}

		var sb strings.Builder
		changed := false
		for i := 0; i < buf.Lines.Len; i++ {
			line := wig.CursorLineByNum(buf, i)
			if line == nil {
				break
			}
			text := string(line.Value)
			hasNewline := strings.HasSuffix(text, "\n")
			content := strings.TrimSuffix(text, "\n")

			if matchSet[i] && strings.Contains(content, query) {
				content = strings.ReplaceAll(content, query, replacement)
				totalLines++
				changed = true
			}

			sb.WriteString(content)
			if hasNewline {
				sb.WriteString("\n")
			}
		}

		if !changed {
			continue
		}

		// Reload the buffer transactionally so undo/redo history is
		// preserved and the git gutter / LSP / highlighter see a proper
		// reload event (mirrors reloadBufferPostFormat in commands.go).
		sub := ctx
		sub.Buf = buf
		wig.ReloadBufferContent(sub, sb.String())
		if buf.Highlighter != nil {
			buf.Highlighter.Build()
		}
		ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: buf})
		buf.Dirty = true
		filesChanged++
	}

	ctx.Editor.EchoMessage(fmt.Sprintf("Replaced %d line(s) across %d file(s)", totalLines, filesChanged))

	rgMu.Lock()
	rgRS = rgReplaceStateT{}
	rgMu.Unlock()
	rgInstallDefaultHandler(ctx.Buf)
	ctx.Editor.Redraw()
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
			wig.CmdCursorCenter(ctx)
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
			wig.CmdCursorCenter(ctx)
			return
		}
	}
}
