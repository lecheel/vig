package rgcollect

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
)

// RgViewWidget renders the [rg] grouped search results as a 100%-screen-width
// overlay that leaves only the bottom statusline (and the echo-message line
// above it) visible. It replaces the previous approach of showing the [rg]
// buffer in a regular window, which required a full-screen takeover of the
// active window (losing split context) or a separate split (taking screen
// real estate away from the file view).
//
// Key handling is intentionally transparent: Mode() and Keymap() delegate to
// the active window's current buffer, so the browse / replace / search sub-
// mode handlers installed on the [rg] buffer (see rgInstallBrowseHandler,
// rgInstallReplaceHandler, rgInstallSearchHandler) keep working unchanged,
// and any popup opened on top (Picker, Confirm, ...) still receives its own
// keys. The widget's only job is painting.
//
// The [rg] buffer is still installed in the active window (see InitGrouped)
// so that ctx.Buf / WindowCursorGet — which every existing rg key handler
// depends on via wig.ContextCursorGet(ctx) — continue to resolve to it. The
// widget paints over the window's own rendering of that buffer, so
// WindowRender's line-number / git-sign / blame gutter never becomes visible
// while the widget is active.
type RgViewWidget struct {
	e   *wig.Editor
	buf *wig.Buffer
}

func (u *RgViewWidget) Plane() wig.RenderPlane { return wig.PlaneEditor }

// Mode delegates to the active window's buffer. When the rg buffer is
// visible, that's the rg buffer's mode (always NORMAL); when the user has
// navigated away, it's whatever the new buffer's mode is. Either way the
// framework looks up the right keymap.
func (u *RgViewWidget) Mode() wig.Mode {
	if w := u.e.ActiveWindow(); w != nil && w.Buffer() != nil {
		return w.Buffer().Mode()
	}
	return wig.MODE_NORMAL
}

// Keymap delegates to the active window's buffer handler so the widget is
// fully transparent for input. The rg buffer's handler (installed by
// rgInstallBrowseHandler / rgInstallReplaceHandler / rgInstallSearchHandler)
// is used whenever the rg buffer is the active one; when the user has
// switched to another buffer, that buffer's handler is used instead and the
// widget simply stops painting (see Render).
func (u *RgViewWidget) Keymap() *wig.KeyHandler {
	if w := u.e.ActiveWindow(); w != nil && w.Buffer() != nil && w.Buffer().KeyHandler != nil {
		return w.Buffer().KeyHandler
	}
	return u.e.Keys
}

// InitRgViewWidget removes any previous RgViewWidget (stale or otherwise)
// and pushes a new one bound to buf. Safe to call repeatedly.
func InitRgViewWidget(e *wig.Editor, buf *wig.Buffer) *RgViewWidget {
	CloseRgViewWidget(e)
	w := &RgViewWidget{e: e, buf: buf}
	e.PushUi(w)
	return w
}

// CloseRgViewWidget removes any RgViewWidget currently on the UI stack.
func CloseRgViewWidget(e *wig.Editor) {
	for i := len(e.UiComponents) - 1; i >= 0; i-- {
		if _, ok := e.UiComponents[i].(*RgViewWidget); ok {
			e.UiComponents = append(e.UiComponents[:i], e.UiComponents[i+1:]...)
		}
	}
}

// Render paints the [rg] buffer's content full screen width inside a
// rounded frame:
//
//   - row  0          : box top edge    ╭───╮
//   - rows 1 .. vh-4  : buffer content  │…  │  (buffer row = cur.ScrollOffset + y - 1)
//   - row  vh-3       : box bottom edge ╰───╯
//   - row  vh-2       : echo message (rgRenderBrowseStatus and friends),
//     cleared when there is no message so stale window
//     content can never bleed through
//   - row  vh-1       : left untouched for the underlying statusline
//
// If the active window's buffer is no longer the rg buffer (e.g. the user
// opened a file with Enter or :cn), the widget paints nothing — the
// underlying window's normal rendering is visible instead.
// rgTruncate returns s truncated to maxLen runes, appending "..." when cut.
// Local to this file because ui.truncate is unexported and rgcollect cannot
// import the ui package.
func rgTruncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-3]) + "..."
}

// rgSearchInfoLine formats the popup's header line:
//
//	search: <query> - <current>/<total> matches in <files> files
//
// current is the 1-based index of the result under the cursor (0 when the
// cursor is not on a result line, e.g. on a file header or blank separator).
// total counts every match span across every result; files counts distinct
// file paths.
func rgSearchInfoLine(curLine int) string {
	title := rgState.title
	if title == "" {
		title = "?"
	}
	total := 0
	filesMap := make(map[string]struct{})
	for _, r := range rgState.results {
		filesMap[r.FilePath] = struct{}{}
		if n := len(r.GetMatches()); n > 0 {
			total += n
		} else {
			total++
		}
	}
	current := 0
	if entry, ok := rgState.lineMap[curLine]; ok && entry.kind == 2 {
		current = entry.resultIdx + 1
	}
	return fmt.Sprintf(" search: %s - %d/%d matches in %d files",
		title, current, total, len(filesMap))
}

// rgReplaceLine formats the popup's replace prompt line:
//
//	Replace: [<text>] <-              (browse / search phases)
//	Replace: [<before>█<after>] <-    (replace phase, cursor as block)
//
// The trailing "<-" is a static indicator pointing at the input box; it is
// not a cursor.
func rgReplaceLine() string {
	if rgRSP.phase == rgPhaseReplace {
		before := string(rgRSP.replacement[:rgRSP.replaceCursor])
		after := string(rgRSP.replacement[rgRSP.replaceCursor:])
		return fmt.Sprintf(" Replace: [%s█%s] <-", before, after)
	}
	return fmt.Sprintf(" Replace: [%s] <-", string(rgRSP.replacement))
}

// Render paints the [rg] buffer's content full screen width inside a
// rounded frame that reserves only the bottom statusline row:
//
//	row  0          ╭─ rg ─────...──────╮     box top edge (with "rg" title)
//	row  1          │ search: … - N/M matches in K files
//	row  2          │ Replace: [ … █ … ] <-
//	rows 3..vh-3    │ <buffer content>        (row = 3 + (lineNum - ScrollOffset))
//	row  vh-2       ╰─ [shortcut hint] ─╯    box bottom edge (hint embedded)
//	row  vh-1       (left untouched — underlying statusline)
//
// This mirrors the git status popup's layout (see
// ui.GitViewPopupWidget.Render): the shortcut hint is written across the
// middle of the bottom border rather than occupying its own row, so the
// content area extends all the way down to the row above it and the frame
// never wastes a row on chrome. Echo messages (e.g. "[3/47 matches] foo"
// from :cn / :cp) fall through to the editor's normal statusline on row
// vh-1 instead of being painted inside the popup.
//
// The buffer itself only contains the result entries (file headers, blank
// separators, match lines) — see InitGrouped.
//
// If the active window's buffer is no longer the rg buffer (e.g. the user
// opened a file with Enter or :cn), the widget paints nothing — the
// underlying window's normal rendering is visible instead.
func (u *RgViewWidget) Render(view wig.View) {
	if w := u.e.ActiveWindow(); w == nil || w.Buffer() != u.buf {
		return
	}
	if u.buf == nil {
		return
	}

	vw, vh := view.Size()
	if vw < 6 || vh < 8 {
		return
	}

	bg := wig.Color("default")

	// Border style: prefer the theme's comment colour, which is subtle and
	// reads as chrome rather than content; fall back to ui.linenr.
	borderStyle := wig.Color("ui.linenr")
	if s, ok := wig.FindColor("comment"); ok {
		borderStyle = s
	}
	infoStyle := wig.Color("ui.text.directory")
	replaceStyle := wig.Color("ui.text")
	hintStyle := wig.Color("ui.linenr")

	// Clear rows 0..vh-2 so stale cells from the window below can never
	// leak through. Row vh-1 is left alone for the statusline.
	for y := 0; y < vh-1; y++ {
		view.SetContent(0, y, strings.Repeat(" ", vw), bg)
	}

	// Row layout mirrors the git status popup: the shortcut hint is
	// embedded in the bottom border row itself (see below), so the
	// content area runs all the way down to the row just above it — no
	// separate status row inside the frame.
	boxTop := 0
	infoRow := 1
	replaceRow := 2
	contentTop := 3
	boxBottom := vh - 2

	contentBottom := boxBottom - 1
	contentH := contentBottom - contentTop + 1
	if contentH < 1 {
		contentH = 1
	}
	contentX := 1
	contentW := vw - 2
	if contentW < 1 {
		contentW = 1
	}

	// Rounded frame.
	view.SetContent(0, boxTop, "╭", borderStyle)
	view.SetContent(vw-1, boxTop, "╮", borderStyle)
	for x := 1; x < vw-1; x++ {
		view.SetContent(x, boxTop, "─", borderStyle)
		view.SetContent(x, boxBottom, "─", borderStyle)
	}
	view.SetContent(0, boxBottom, "╰", borderStyle)
	view.SetContent(vw-1, boxBottom, "╯", borderStyle)
	for y := boxTop + 1; y < boxBottom; y++ {
		view.SetContent(0, y, "│", borderStyle)
		view.SetContent(vw-1, y, "│", borderStyle)
	}

	// Title text on the top border.
	view.SetContent(2, boxTop, " rg ", borderStyle)

	// Shortcut hint embedded in the bottom border, mirroring the git
	// status popup (ui.GitViewPopupWidget.Render): the hint overwrites
	// the middle of the ╰───╯ run instead of occupying a row of its own.
	view.SetContent(2, boxBottom, rgTruncate(rgBrowseHint, vw-4), hintStyle)

	cur := wig.WindowCursorGet(u.e.ActiveWindow(), u.buf)
	if cur == nil {
		return
	}

	// Header: search stats.
	view.SetContent(1, infoRow, rgTruncate(rgSearchInfoLine(cur.Line), contentW), infoStyle)

	// Header: replace prompt. Highlight it when the replace sub-mode is
	// active so the user has a clear cue that keystrokes now edit the
	// replacement.
	if rgRSP.phase == rgPhaseReplace {
		if s, ok := wig.FindColor("ui.menu.selected"); ok {
			replaceStyle = s
		}
	}
	view.SetContent(1, replaceRow, rgTruncate(rgReplaceLine(), contentW), replaceStyle)

	// Content area — same buffer row -> screen row mapping as before, just
	// shifted down by contentTop instead of being offset by the frame.
	hl, _ := u.buf.Highlighter.(*RgHighlighter)

	if cur.Line < cur.ScrollOffset {
		cur.ScrollOffset = cur.Line
	}
	if cur.Line >= cur.ScrollOffset+contentH {
		cur.ScrollOffset = cur.Line - contentH + 1
	}
	if cur.ScrollOffset < 0 {
		cur.ScrollOffset = 0
	}

	cursorStyle := wig.Color("ui.cursor")
	if c, ok := wig.FindColor("ui.selection"); ok {
		cursorStyle = c
	}
	if c, ok := wig.FindColor("ui.cursor.primary"); ok {
		cursorStyle = c
	}

	lineNum := 0
	line := u.buf.Lines.First()
	for line != nil {
		relY := lineNum - cur.ScrollOffset
		if relY >= 0 && relY < contentH {
			y := contentTop + relY
			runes := line.Value

			var spans []wig.Span
			if hl != nil {
				spans = hl.HighlightLine(lineNum)
			}

			spanIdx := 0
			x := 0
			for i := 0; i < len(runes) && x < contentW; i++ {
				ch := runes[i]
				if ch == '\n' {
					break
				}

				st := bg
				for spanIdx < len(spans) && int(spans[spanIdx].EndCol) <= i {
					spanIdx++
				}
				if spanIdx < len(spans) &&
					int(spans[spanIdx].StartCol) <= i &&
					int(spans[spanIdx].EndCol) > i {
					st = spans[spanIdx].Style
				}

				cellWidth := 1
				if ch == '\t' {
					cellWidth = 4
				}

				if lineNum == cur.Line && i == cur.Char {
					st = cursorStyle
				}

				if ch == '\t' {
					for k := 0; k < cellWidth && x+k < contentW; k++ {
						view.SetContent(contentX+x+k, y, " ", st)
					}
				} else {
					view.SetContent(contentX+x, y, string(ch), st)
				}
				x += cellWidth
			}

			if lineNum == cur.Line && cur.Char >= len(runes)-1 && x < contentW {
				view.SetContent(contentX+x, y, " ", cursorStyle)
			}
		}
		line = line.Next()
		lineNum++
	}

}
