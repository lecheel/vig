package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// =============================================================================
// Diff model
// =============================================================================

// HunkDiffHunk is a contiguous region of difference between the working
// buffer (left) and HEAD (right). LeftFrom/LeftTo/RightFrom/RightTo are
// 1-based and inclusive.
type HunkDiffHunk struct {
	LeftFrom, LeftTo   int
	RightFrom, RightTo int
}

type hunkDiffRowKind int

const (
	hunkDiffContext hunkDiffRowKind = iota
	hunkDiffDelete
	hunkDiffInsert
)

// HunkDiffRow is one aligned row. LeftIdx / RightIdx are 0-based indices
// into the working / HEAD lines, or -1 when that side is empty. HunkIdx is
// the index into Hunks, or -1 for context rows.
type HunkDiffRow struct {
	LeftIdx, RightIdx int
	Kind              hunkDiffRowKind
	HunkIdx           int
}

type HunkDiffData struct {
	Hunks []HunkDiffHunk
	Rows  []HunkDiffRow
}

// computeHunkDiff produces an LCS-aligned diff between work and head.
func computeHunkDiff(work, head []string) *HunkDiffData {
	m, n := len(work), len(head)
	if m == 0 && n == 0 {
		return &HunkDiffData{}
	}

	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		wi := work[i]
		for j := n - 1; j >= 0; j-- {
			if wi == head[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	d := &HunkDiffData{}
	i, j := 0, 0
	for i < m || j < n {
		if i < m && j < n && work[i] == head[j] {
			d.Rows = append(d.Rows, HunkDiffRow{
				LeftIdx: i, RightIdx: j,
				Kind: hunkDiffContext, HunkIdx: -1,
			})
			i++
			j++
			continue
		}

		startRow := len(d.Rows)
		leftStart, rightStart := i, j

		for i < m || j < n {
			if i < m && j < n && work[i] == head[j] {
				break
			}
			if j >= n || (i < m && lcs[i+1][j] >= lcs[i][j+1]) {
				d.Rows = append(d.Rows, HunkDiffRow{
					LeftIdx: i, RightIdx: -1,
					Kind: hunkDiffDelete, HunkIdx: -1,
				})
				i++
			} else {
				d.Rows = append(d.Rows, HunkDiffRow{
					LeftIdx: -1, RightIdx: j,
					Kind: hunkDiffInsert, HunkIdx: -1,
				})
				j++
			}
		}

		hunkIdx := len(d.Hunks)
		d.Hunks = append(d.Hunks, HunkDiffHunk{
			LeftFrom: leftStart + 1, LeftTo: i,
			RightFrom: rightStart + 1, RightTo: j,
		})
		for k := startRow; k < len(d.Rows); k++ {
			d.Rows[k].HunkIdx = hunkIdx
		}
	}

	return d
}

// =============================================================================
// Widget
//
// UiHunkDiff is a standalone fullscreen UiComponent. It does not inherit
// any editor keys: its Keymap() returns a fresh handler with only the keys
// listed here bound, and a no-op fallback so unbound keys can never leak
// into the underlying buffer.
//
// Everything is drawn directly by Render() into the main view (PlaneEditor).
// To make that safe on every frame the renderer repaints, Render() writes
// *every* cell of the viewport one at a time — borders, panels, content,
// status — so no stale cell from the underlying window can leak through.
// =============================================================================

type UiHunkDiff struct {
	e      *wig.Editor
	keymap *wig.KeyHandler
	buf    *wig.Buffer // file being diffed; edits go through this
	head   []string
	d      *HunkDiffData
	cur    int
	scroll int
	focus  string // "left" or "right"
	status string
}

func (u *UiHunkDiff) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *UiHunkDiff) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *UiHunkDiff) Keymap() *wig.KeyHandler { return u.keymap }

// workLines reads the file buffer's current content as a slice of strings
// with trailing newlines stripped.
func (u *UiHunkDiff) workLines() []string {
	if u.buf == nil {
		return nil
	}
	var lines []string
	l := u.buf.Lines.First()
	for l != nil {
		lines = append(lines, strings.TrimSuffix(string(l.Value), "\n"))
		l = l.Next()
	}
	return lines
}

// refresh recomputes the diff from the current file buffer contents.
func (u *UiHunkDiff) refresh() {
	u.d = computeHunkDiff(u.workLines(), u.head)
	u.clampCursor()
}

func (u *UiHunkDiff) clampCursor() {
	if u.cur < 0 {
		u.cur = 0
	}
	if u.cur >= len(u.d.Rows) {
		u.cur = len(u.d.Rows) - 1
	}
	if u.cur < 0 {
		u.cur = 0
	}
}

// currentHunk returns the hunk under the cursor, if any.
func (u *UiHunkDiff) currentHunk() (HunkDiffHunk, bool) {
	if u.cur < 0 || u.cur >= len(u.d.Rows) {
		return HunkDiffHunk{}, false
	}
	idx := u.d.Rows[u.cur].HunkIdx
	if idx < 0 || idx >= len(u.d.Hunks) {
		return HunkDiffHunk{}, false
	}
	return u.d.Hunks[idx], true
}

// setBufferLines replaces the file buffer's content as one undo transaction
// and refreshes the diff. ReloadBufferContent keeps undo/redo, git gutter
// and LSP seeing a proper reload event, but it does NOT rebuild the
// syntax highlighter (see commands.reloadBufferPostFormat, which does this
// explicitly), so we do the same here — otherwise the tree-sitter spans
// stay anchored to the pre-edit line contents and the render path picks
// up stale styles.
func (u *UiHunkDiff) setBufferLines(ctx wig.Context, lines []string) {
	sub := ctx
	sub.Buf = u.buf
	wig.ReloadBufferContent(sub, strings.Join(lines, "\n"))
	if u.buf.Highlighter != nil {
		u.buf.Highlighter.Build()
	}
	ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: u.buf})
	u.refresh()
}

// =============================================================================
// Commands
// =============================================================================

// CmdHunkDiffOpen opens the two-panel hunk diff for the active buffer.
func CmdHunkDiffOpen(ctx wig.Context) {
	if ctx.Buf == nil || ctx.Buf.FilePath == "" || strings.HasPrefix(ctx.Buf.FilePath, "[") {
		ctx.Editor.EchoMessage("No file to diff")
		return
	}
	// Only one open at a time.
	for _, c := range ctx.Editor.UiComponents {
		if _, ok := c.(*UiHunkDiff); ok {
			return
		}
	}
	head, err := hunkDiffHeadLines(ctx.Editor, ctx.Buf)
	if err != nil || head == nil {
		ctx.Editor.EchoMessage("No HEAD version for this file")
		return
	}

	u := &UiHunkDiff{
		e:     ctx.Editor,
		buf:   ctx.Buf,
		head:  head,
		focus: "left",
	}
	u.refresh()
	// Start on first hunk.
	for i, r := range u.d.Rows {
		if r.HunkIdx >= 0 {
			u.cur = i
			break
		}
	}

	u.keymap = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Esc": u.close,
			"q":   u.close,

			"j":    u.down,
			"Down": u.down,
			"k":    u.up,
			"Up":   u.up,

			"g": wig.KeyMap{
				"g": u.goTop,
			},
			"G": u.goBottom,

			"PgDn":   func(ctx wig.Context) { u.moveBy(ctx, +10) },
			"ctrl+d": func(ctx wig.Context) { u.moveBy(ctx, +10) },
			"PgUp":   func(ctx wig.Context) { u.moveBy(ctx, -10) },
			"ctrl+u": func(ctx wig.Context) { u.moveBy(ctx, -10) },

			"n": u.nextHunk,
			"l": u.nextHunk,
			"]": u.nextHunk,
			"N": u.prevHunk,
			"L": u.prevHunk,
			"[": u.prevHunk,

			"Tab": u.toggleFocus,

			"a": u.applyHunk,
			"A": u.applyAll,
			"d": u.deleteLine,
			"y": u.yank,
			"p": u.paste,
			"P": u.paste,
			"u": u.undo,
			"w": u.write,
		},
	})
	// Block every unbound key so it can never reach the editor / buffer.
	u.keymap.Fallback(func(ctx wig.Context, ev *tcell.EventKey) {})

	ctx.Editor.PushUi(u)
	ctx.Editor.Redraw()
}

// CmdHunkDiffClose closes the hunk diff view if it is open.
func CmdHunkDiffClose(ctx wig.Context) {
	for i := len(ctx.Editor.UiComponents) - 1; i >= 0; i-- {
		if _, ok := ctx.Editor.UiComponents[i].(*UiHunkDiff); ok {
			ctx.Editor.PopUiComponent(ctx.Editor.UiComponents[i])
			ctx.Editor.Redraw()
			return
		}
	}
}

// =============================================================================
// Actions
// =============================================================================

func (u *UiHunkDiff) close(ctx wig.Context) {
	ctx.Editor.PopUiComponent(u)
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) down(ctx wig.Context)  { u.cur++; u.clampCursor(); ctx.Editor.Redraw() }
func (u *UiHunkDiff) up(ctx wig.Context)    { u.cur--; u.clampCursor(); ctx.Editor.Redraw() }
func (u *UiHunkDiff) goTop(ctx wig.Context) { u.cur = 0; u.scroll = 0; ctx.Editor.Redraw() }
func (u *UiHunkDiff) goBottom(ctx wig.Context) {
	u.cur = len(u.d.Rows) - 1
	u.clampCursor()
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) moveBy(ctx wig.Context, n int) {
	u.cur += n
	u.clampCursor()
	ctx.Editor.Redraw()
}

// nextHunk / prevHunk walk hunk-by-hunk, wrapping around like ]c / [c.
func (u *UiHunkDiff) nextHunk(ctx wig.Context) {
	if len(u.d.Hunks) == 0 {
		return
	}
	startHunk := u.d.Rows[u.cur].HunkIdx
	if startHunk < 0 {
		for i := u.cur + 1; i < len(u.d.Rows); i++ {
			if u.d.Rows[i].HunkIdx >= 0 {
				u.cur = i
				ctx.Editor.Redraw()
				return
			}
		}
		for i := 0; i < len(u.d.Rows); i++ {
			if u.d.Rows[i].HunkIdx >= 0 {
				u.cur = i
				ctx.Editor.Redraw()
				return
			}
		}
		return
	}
	next := (startHunk + 1) % len(u.d.Hunks)
	for i, r := range u.d.Rows {
		if r.HunkIdx == next {
			u.cur = i
			ctx.Editor.Redraw()
			return
		}
	}
}

func (u *UiHunkDiff) prevHunk(ctx wig.Context) {
	if len(u.d.Hunks) == 0 {
		return
	}
	startHunk := u.d.Rows[u.cur].HunkIdx
	if startHunk < 0 {
		for i := u.cur - 1; i >= 0; i-- {
			if u.d.Rows[i].HunkIdx >= 0 {
				u.cur = i
				ctx.Editor.Redraw()
				return
			}
		}
		for i := len(u.d.Rows) - 1; i >= 0; i-- {
			if u.d.Rows[i].HunkIdx >= 0 {
				u.cur = i
				ctx.Editor.Redraw()
				return
			}
		}
		return
	}
	prev := startHunk - 1
	if prev < 0 {
		prev = len(u.d.Hunks) - 1
	}
	for i, r := range u.d.Rows {
		if r.HunkIdx == prev {
			u.cur = i
			ctx.Editor.Redraw()
			return
		}
	}
}

func (u *UiHunkDiff) toggleFocus(ctx wig.Context) {
	if u.focus == "right" {
		u.focus = "left"
	} else {
		u.focus = "right"
	}
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) yank(ctx wig.Context) {
	if u.d == nil || u.cur < 0 || u.cur >= len(u.d.Rows) {
		return
	}
	row := u.d.Rows[u.cur]
	var line string
	if u.focus == "right" {
		if row.RightIdx < 0 || row.RightIdx >= len(u.head) {
			u.status = "Nothing to yank here"
			ctx.Editor.Redraw()
			return
		}
		line = u.head[row.RightIdx]
	} else {
		work := u.workLines()
		if row.LeftIdx < 0 || row.LeftIdx >= len(work) {
			u.status = "Nothing to yank here"
			ctx.Editor.Redraw()
			return
		}
		line = work[row.LeftIdx]
	}
	text := line + "\n"

	// Mirror the reference hunkdiff behavior: put the line into the
	// system clipboard as well as wig's own registers, so it can be
	// pasted into other apps. wig's saveRegister only writes to the
	// system clipboard when the "active register" is '+' or '*', so we
	// write directly here instead of going through SetRegister.
	_ = clipboard.WriteAll(text)

	// '0' = dedicated yank register, '"' = unnamed register. Both are
	// marked line-kind so a subsequent `p` in any wig buffer (including
	// this widget's paste) treats them as whole-line content.
	wig.SetRegister('0', text, true, false)
	wig.SetRegister('"', text, true, false)
	wig.SetRegister('+', text, true, false)
	wig.SetRegister('*', text, true, false)

	u.status = "Yanked 1 line"
	ctx.Editor.Redraw()
}

// paste inserts the unnamed register's content below the cursor's
// working-side row.
//
// Matching the reference hunkdiff semantics, paste only accepts line-kind
// content: a yank of a character or a partial line must NOT paste as a
// whole line (that would silently replace or duplicate text the user
// never selected). The line-kind flag is carried by wig's `yank` struct
// and surfaced through NamedRegisters.
//
// The register value may span multiple lines (e.g. `3yy` produces one
// entry with embedded \n). We split and insert each line as a separate
// buffer line so embedded newlines never end up inside a single element.
func (u *UiHunkDiff) paste(ctx wig.Context) {
	if u.d == nil || u.cur < 0 || u.cur >= len(u.d.Rows) {
		return
	}

	// Line-kind check: the unnamed register must be a line-kind yank.
	// A missing register, or one that is stream-kind (character / range
	// yank), is rejected just like the reference implementation.
	reg, ok := wig.NamedRegisters['"']
	if !ok || !reg.IsLine {
		u.status = "Clipboard has no line"
		ctx.Editor.Redraw()
		return
	}

	text := strings.TrimRight(reg.Val, "\r\n")
	if text == "" {
		u.status = "Clipboard is empty"
		ctx.Editor.Redraw()
		return
	}
	insertLines := strings.Split(text, "\n")

	work := u.workLines()
	row := u.d.Rows[u.cur]
	insertAt := row.LeftIdx + 1
	if row.LeftIdx < 0 {
		// Padding row on the left: paste at the next real left line.
		for r := u.cur - 1; r >= 0; r-- {
			if u.d.Rows[r].LeftIdx >= 0 {
				insertAt = u.d.Rows[r].LeftIdx + 1
				break
			}
		}
		if insertAt < 0 {
			insertAt = 0
		}
	}
	if insertAt > len(work) {
		insertAt = len(work)
	}
	newLines := make([]string, 0, len(work)+len(insertLines))
	newLines = append(newLines, work[:insertAt]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, work[insertAt:]...)
	u.setBufferLines(ctx, newLines)
	u.status = "Pasted line"
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) deleteLine(ctx wig.Context) {
	if u.focus == "right" {
		u.status = "Cannot delete from HEAD"
		ctx.Editor.Redraw()
		return
	}
	row := u.d.Rows[u.cur]
	if row.LeftIdx < 0 {
		u.status = "Nothing to delete here"
		ctx.Editor.Redraw()
		return
	}
	work := u.workLines()
	idx := row.LeftIdx
	if idx < 0 || idx >= len(work) {
		return
	}
	newLines := make([]string, 0, len(work)-1)
	newLines = append(newLines, work[:idx]...)
	newLines = append(newLines, work[idx+1:]...)
	if len(newLines) == 0 {
		newLines = []string{""}
	}
	u.setBufferLines(ctx, newLines)
	u.status = "Deleted line"
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) applyHunk(ctx wig.Context) {
	h, ok := u.currentHunk()
	if !ok {
		u.status = "Cursor is not on a hunk"
		ctx.Editor.Redraw()
		return
	}
	work := u.workLines()
	leftStart := h.LeftFrom - 1
	leftEnd := h.LeftTo
	rightStart := h.RightFrom - 1
	rightEnd := h.RightTo
	if leftStart < 0 {
		leftStart = 0
	}
	if leftEnd > len(work) {
		leftEnd = len(work)
	}
	if rightStart < 0 {
		rightStart = 0
	}
	if rightEnd > len(u.head) {
		rightEnd = len(u.head)
	}
	newLines := make([]string, 0, len(work)-(leftEnd-leftStart)+(rightEnd-rightStart))
	newLines = append(newLines, work[:leftStart]...)
	newLines = append(newLines, u.head[rightStart:rightEnd]...)
	newLines = append(newLines, work[leftEnd:]...)
	if len(newLines) == 0 {
		newLines = []string{""}
	}
	u.setBufferLines(ctx, newLines)
	u.status = "Applied hunk"
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) applyAll(ctx wig.Context) {
	if len(u.d.Hunks) == 0 {
		u.status = "No hunks to apply"
		ctx.Editor.Redraw()
		return
	}
	u.setBufferLines(ctx, append([]string(nil), u.head...))
	u.status = "Applied all hunks"
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) undo(ctx wig.Context) {
	if u.buf == nil || u.buf.UndoRedo == nil || u.buf.UndoRedo.Position < 0 {
		u.status = "Nothing to undo"
		ctx.Editor.Redraw()
		return
	}
	u.buf.UndoRedo.Undo()
	ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: u.buf})
	u.refresh()
	u.status = "Undo"
	ctx.Editor.Redraw()
}

func (u *UiHunkDiff) write(ctx wig.Context) {
	sub := ctx
	sub.Buf = u.buf
	sub.Char = ""
	CmdSaveFileWithFeedback(sub)
	u.status = "File written"
	ctx.Editor.Redraw()
}

// =============================================================================
// Helpers
// =============================================================================

// hunkDiffHeadLines returns the file's content at HEAD as a slice of lines
// (no trailing newline). Errors when the file is untracked or HEAD missing.
func hunkDiffHeadLines(e *wig.Editor, buf *wig.Buffer) ([]string, error) {
	if buf.FilePath == "" || strings.HasPrefix(buf.FilePath, "[") {
		return nil, fmt.Errorf("not a real file")
	}
	rootDir, err := e.Projects.FindRoot(buf)
	if err != nil {
		return nil, err
	}
	relPath := strings.TrimPrefix(buf.FilePath, rootDir+"/")
	if relPath == "" || relPath == buf.FilePath {
		return nil, fmt.Errorf("not in git repo")
	}
	cmd := exec.Command("git", "show", "HEAD:"+relPath)
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	s := strings.TrimSuffix(string(out), "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

// renderSourceLine writes a buffer source line into the view at (x, y),
// truncated/padded to exactly w visual cells, applying syntax-highlight
// spans from hl on top of baseStyle.
//
// lineNum is the 0-based line index in the buffer that hl was built for
// (pass -1 to skip highlighting, e.g. for HEAD-only insert rows that have
// no corresponding work-buffer line). Spans are indexed by rune position,
// matching the convention ui.WindowRender uses when it consumes the same
// highlighter — see the span-walk loop in ui/window.go.
//
// Tab characters are expanded inline to 4 spaces during the write, so the
// per-cell width accounting stays exact even though tabs are one rune but
// several cells. Control bytes (everything below 0x20 except tab, plus DEL)
// are skipped: they can appear in a source file from a pasted terminal
// capture, and writing them verbatim into the cell buffer would corrupt
// neighboring cells on some terminals.
//
// Highlight layering: we take the foreground from the span's style and
// keep the background from baseStyle. That preserves the diff backgrounds
// (diff.minus / diff.plus) and the cursor-line background under the
// highlighted text, instead of letting the span's own background overwrite
// them and break the diff visual.
func renderSourceLine(
	view wig.View,
	x, y, w int,
	raw string,
	lineNum int,
	hl wig.Highlighter,
	baseStyle tcell.Style,
) {
	if w <= 0 {
		return
	}

	var spans []wig.Span
	if hl != nil && lineNum >= 0 {
		spans = hl.HighlightLine(lineNum)
	}

	// Base background, re-applied whenever we layer a span's foreground.
	_, baseBg, _ := baseStyle.Decompose()

	col := 0
	spanIdx := 0
	runeIdx := 0

	for _, ch := range raw {
		// Skip control bytes that should never reach the cell buffer.
		if (ch < 0x20 && ch != '\t') || ch == 0x7f {
			runeIdx++
			continue
		}

		// Advance past spans that have already ended.
		for spanIdx < len(spans) && int(spans[spanIdx].EndCol) <= runeIdx {
			spanIdx++
		}
		cellStyle := baseStyle
		if spanIdx < len(spans) &&
			int(spans[spanIdx].StartCol) <= runeIdx &&
			int(spans[spanIdx].EndCol) > runeIdx {
			fg, _, _ := spans[spanIdx].Style.Decompose()
			if fg != tcell.ColorDefault {
				cellStyle = tcell.StyleDefault.Background(baseBg).Foreground(fg)
			}
		}

		// Width of this rune in cells. Tabs expand to 4; wide runes
		// (CJK, emoji) take their measured width; zero-width combining
		// marks are treated as 1 to avoid an infinite loop.
		cellWidth := 1
		if ch == '\t' {
			cellWidth = 4
		} else {
			cellWidth = runewidth.RuneWidth(ch)
			if cellWidth <= 0 {
				cellWidth = 1
			}
		}
		if col+cellWidth > w {
			cellWidth = w - col
		}
		if cellWidth <= 0 {
			break
		}

		if ch == '\t' {
			for k := 0; k < cellWidth; k++ {
				view.SetContent(x+col+k, y, " ", cellStyle)
			}
		} else {
			view.SetContent(x+col, y, string(ch), cellStyle)
			for k := 1; k < cellWidth; k++ {
				view.SetContent(x+col+k, y, " ", cellStyle)
			}
		}
		col += cellWidth
		runeIdx++
		if col >= w {
			break
		}
	}

	// Pad the remainder of the panel with baseStyle so no stale cell from
	// the previous frame can leak through.
	for ; col < w; col++ {
		view.SetContent(x+col, y, " ", baseStyle)
	}
}

// fillRow writes n blank cells starting at (x, y). Used to clear regions of
// the viewport cell-by-cell so the count of cells written exactly matches
// the region size.
func fillRow(view wig.View, x, y, n int, style tcell.Style) {
	for i := 0; i < n; i++ {
		view.SetContent(x+i, y, " ", style)
	}
}

// hunkBgFor returns the background color used to shade rows belonging to a
// hunk, giving the change region a block visual rather than colored text
// alone. Two colors distinguish side:
//
//   - deleteSide=true:  the reddish tint applied to delete rows
//   - deleteSide=false: the greenish tint applied to insert rows
//
// Both panels of a hunk row share the same tint, so a row reads as a single
// band spanning the split column. Adjacent delete/insert rows within the
// same hunk still show the classic red/green distinction because the tint
// changes between them.
//
// Resolution order, most to least preferred:
//
//  1. Theme key `diff.hunk.delete` / `diff.hunk.insert`, if it defines a
//     background. Themes can opt in to a custom hunk tint here.
//  2. The theme's own `diff.minus` / `diff.plus` background, if set. Many
//     themes already ship diff colors with a background, and reusing it
//     keeps the hunk block consistent with how the rest of the editor
//     paints diffs.
//  3. A low-weight blend of the theme's `diff.minus` / `diff.plus`
//     foreground into the default background, so the tint tracks the
//     palette even when the theme sets no backgrounds at all.
//  4. ColorDefault — no shading. Only happens when the theme defines
//     neither the hunk keys, nor a real default bg, nor a diff fg.
//
// The blend weight is deliberately low (3/16). Syntax foregrounds and the
// cursor-line highlight layer on top of this background, and shouldn't be
// drowned out by the hunk tint.
func hunkBgFor(deleteSide bool) tcell.Color {
	hunkKey := "diff.hunk.insert"
	accentKey := "diff.plus"
	if deleteSide {
		hunkKey = "diff.hunk.delete"
		accentKey = "diff.minus"
	}

	// 1. Explicit hunk key.
	if s, ok := wig.FindColor(hunkKey); ok {
		_, bg, _ := s.Decompose()
		if bg != tcell.ColorDefault {
			return bg
		}
	}

	// 2. Theme's own diff.minus/diff.plus background.
	if s, ok := wig.FindColor(accentKey); ok {
		_, bg, _ := s.Decompose()
		if bg != tcell.ColorDefault {
			return bg
		}
	}

	// 3. Blend the accent fg into the default bg.
	_, defaultBg, _ := wig.Color("default").Decompose()
	if defaultBg == tcell.ColorDefault {
		return tcell.ColorDefault
	}
	accentFg, _, _ := wig.Color(accentKey).Decompose()
	if accentFg == tcell.ColorDefault {
		return tcell.ColorDefault
	}
	dr, dg, db := defaultBg.RGB()
	ar, ag, ab := accentFg.RGB()
	const w = int32(3) // out of 16 — subtle
	nr := (dr*(16-w) + ar*w) / 16
	ng := (dg*(16-w) + ag*w) / 16
	nb := (db*(16-w) + ab*w) / 16
	return tcell.NewRGBColor(nr, ng, nb)
}

// writeCellRow writes s starting at (x, y), padded/truncated to exactly n
// visual cells. Width is tracked with runewidth so wide runes (CJK, emoji)
// don't desync the padding from what the terminal actually draws.
func writeCellRow(view wig.View, x, y, n int, s string, style tcell.Style) {
	if n <= 0 {
		return
	}
	col := 0
	for _, ch := range s {
		w := runewidth.RuneWidth(ch)
		if w == 0 {
			w = 1
		}
		if col+w > n {
			break
		}
		view.SetContent(x+col, y, string(ch), style)
		col += w
	}
	for ; col < n; col++ {
		view.SetContent(x+col, y, " ", style)
	}
}

// =============================================================================
// Render
//
// Layout (all cells of the viewport are written every frame):
//
//	y=0             top border with " Hunk Diff - <file> " in the middle
//	y=1             panel headers ("Working (buffer)" | "HEAD")
//	y=2             horizontal divider, with "+" where the split column meets
//	y=3 .. vh-3     aligned diff rows
//	y=vh-2          status / key-hint row
//	y=vh-1          bottom border
//
// Columns:
//
//	x=0, x=vw-1     outer border
//	splitX          panel split column
//	[1, splitX-1]   left panel
//	[splitX+1, vw-2] right panel
//
// ASCII box drawing ("+", "-", "|") is used deliberately — Unicode box
// glyphs are East Asian Ambiguous and terminals disagree on their width,
// which shifts every following cell on the row and produces a corrupted
// layout on some terminals.
// =============================================================================

func (u *UiHunkDiff) Render(view wig.View) {
	vw, vh := view.Size()
	if vw < 20 || vh < 8 {
		return
	}

	bgStyle := wig.Color("default")
	borderStyle := wig.Color("ui.linenr")
	titleStyle := wig.Color("ui.popup.title")
	headerStyle := wig.Color("comment")
	statusStyle := wig.Color("ui.statusline")
	cursorStyle := wig.Color("ui.cursor")
	dividerStyle := wig.Color("ui.linenr")

	// 1. Clear the entire viewport cell-by-cell.
	for y := 0; y < vh; y++ {
		fillRow(view, 0, y, vw, bgStyle)
	}

	// 2. Box border.
	for x := 1; x < vw-1; x++ {
		view.SetContent(x, 0, "-", borderStyle)
		view.SetContent(x, vh-1, "-", borderStyle)
	}
	for y := 1; y < vh-1; y++ {
		view.SetContent(0, y, "|", borderStyle)
		view.SetContent(vw-1, y, "|", borderStyle)
	}
	view.SetContent(0, 0, "+", borderStyle)
	view.SetContent(vw-1, 0, "+", borderStyle)
	view.SetContent(0, vh-1, "+", borderStyle)
	view.SetContent(vw-1, vh-1, "+", borderStyle)

	// 3. Title on top border.
	title := fmt.Sprintf(" Hunk Diff - %s ", u.buf.GetName())
	writeCellRow(view, 3, 0, vw-6, title, titleStyle)

	// 4. Split column.
	splitX := vw / 2
	if splitX < 12 {
		splitX = 12
	}
	if splitX > vw-12 {
		splitX = vw - 12
	}
	leftInner := splitX - 1       // cells in [1, splitX-1]
	rightInner := vw - splitX - 2 // cells in [splitX+1, vw-2]

	// 5. Header row (y=1).
	writeCellRow(view, 1, 1, leftInner, " Working (buffer)", headerStyle)
	view.SetContent(splitX, 1, "|", borderStyle)
	writeCellRow(view, splitX+1, 1, rightInner, " HEAD", headerStyle)

	// 6. Divider row (y=2).
	for x := 1; x < vw-1; x++ {
		if x == splitX {
			view.SetContent(x, 2, "+", dividerStyle)
		} else {
			view.SetContent(x, 2, "-", dividerStyle)
		}
	}

	// 7. Content region.
	rowTop := 3
	rowBottom := vh - 3
	pageSize := rowBottom - rowTop + 1
	if pageSize < 1 {
		pageSize = 1
	}

	// Auto-scroll.
	if u.cur < u.scroll {
		u.scroll = u.cur
	}
	if u.cur >= u.scroll+pageSize {
		u.scroll = u.cur - pageSize + 1
	}
	if u.scroll < 0 {
		u.scroll = 0
	}

	work := u.workLines()

	for i := 0; i < pageSize; i++ {
		rowIdx := u.scroll + i
		y := rowTop + i

		if rowIdx >= len(u.d.Rows) {
			// Empty filler row: panels stay clear.
			fillRow(view, 1, y, leftInner, bgStyle)
			view.SetContent(splitX, y, "|", borderStyle)
			fillRow(view, splitX+1, y, rightInner, bgStyle)
			continue
		}

		row := u.d.Rows[rowIdx]
		leftStyle := bgStyle
		rightStyle := bgStyle

		// Hunk rows get a shaded background so the changed region reads
		// as a block across the split, not just as colored text. The
		// tint depends on the row kind (reddish for delete, greenish
		// for insert); both panels share the same tint so the row is
		// one continuous band even where one panel is empty padding.
		//
		// The content panel keeps its diff.minus / diff.plus foreground
		// on top of the tinted background; the empty panel gets the
		// same background with default foreground so it extends the
		// block without looking like unrendered text.
		hunkBg := tcell.ColorDefault
		if row.HunkIdx >= 0 {
			switch row.Kind {
			case hunkDiffDelete:
				hunkBg = hunkBgFor(true)
				leftStyle = wig.Color("diff.minus").Background(hunkBg)
				rightStyle = wig.Color("default").Background(hunkBg)
			case hunkDiffInsert:
				hunkBg = hunkBgFor(false)
				rightStyle = wig.Color("diff.plus").Background(hunkBg)
				leftStyle = wig.Color("default").Background(hunkBg)
			default:
				// Defensive: context rows always have HunkIdx == -1,
				// but if this ever fires, shade both sides uniformly
				// rather than letting a hunk row go unshaded.
				hunkBg = hunkBgFor(true)
				leftStyle = leftStyle.Background(hunkBg)
				rightStyle = rightStyle.Background(hunkBg)
			}
		}

		isCursorRow := rowIdx == u.cur
		if isCursorRow {
			leftStyle = wig.ApplyBg("ui.cursorline", leftStyle)
			rightStyle = wig.ApplyBg("ui.cursorline", rightStyle)
		}

		// Left panel. The work buffer's line index is exactly the line
		// index the tree-sitter highlighter was built for, so spans line
		// up perfectly. Leading space mirrors the previous layout.
		if row.LeftIdx >= 0 && row.LeftIdx < len(work) {
			view.SetContent(1, y, " ", leftStyle)
			renderSourceLine(view, 2, y, leftInner-1, work[row.LeftIdx], row.LeftIdx, u.buf.Highlighter, leftStyle)
		} else {
			fillRow(view, 1, y, leftInner, leftStyle)
		}

		// Separator column. When the row is inside a hunk, tint the
		// separator with the same background so the band is unbroken
		// across the split column instead of looking like two separate
		// blocks.
		sepStyle := borderStyle
		if hunkBg != tcell.ColorDefault {
			sepStyle = borderStyle.Background(hunkBg)
		}
		view.SetContent(splitX, y, "|", sepStyle)

		// Right panel. For context rows work[i] == head[j], so the work
		// buffer's highlighter at index LeftIdx produces correct spans
		// for the HEAD content too. For pure insert rows there is no
		// corresponding work line, so hlLine stays -1 and the render
		// falls back to baseStyle (plain text on the insert background).
		if row.RightIdx >= 0 && row.RightIdx < len(u.head) {
			view.SetContent(splitX+1, y, " ", rightStyle)
			hlLine := -1
			if row.LeftIdx >= 0 && row.LeftIdx < len(work) {
				hlLine = row.LeftIdx
			}
			renderSourceLine(view, splitX+2, y, rightInner-1, u.head[row.RightIdx], hlLine, u.buf.Highlighter, rightStyle)
		} else {
			fillRow(view, splitX+1, y, rightInner, rightStyle)
		}

		// Cursor row focus indicator.
		if isCursorRow {
			if u.focus == "right" && rightInner > 0 {
				view.SetContent(vw-2, y, " ", cursorStyle)
			} else if leftInner > 0 {
				view.SetContent(1, y, " ", cursorStyle)
			}
		}
	}

	// 8. Status row (y = vh-2).
	msg := u.status
	if msg == "" {
		hunkNum := 0
		if u.cur >= 0 && u.cur < len(u.d.Rows) && u.d.Rows[u.cur].HunkIdx >= 0 {
			hunkNum = u.d.Rows[u.cur].HunkIdx + 1
		}
		msg = fmt.Sprintf(
			" Hunk %d/%d  focus:%s  [ l/L ] hunk  [Tab] focus  [a]pply  [d]el  [y]ank  [p]aste  [u]ndo  [w]save  [q]close",
			hunkNum, len(u.d.Hunks), u.focus,
		)
	} else {
		msg = " " + msg
	}
	writeCellRow(view, 1, vh-2, vw-2, msg, statusStyle)
}
