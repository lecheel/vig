package rgcollect

import (
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
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
func (u *RgViewWidget) Render(view wig.View) {
	if w := u.e.ActiveWindow(); w == nil || w.Buffer() != u.buf {
		return
	}
	if u.buf == nil {
		return
	}

	vw, vh := view.Size()
	if vw < 4 || vh < 4 {
		return
	}

	bg := wig.Color("default")

	// Border style: prefer the theme's comment colour, which is subtle and
	// reads as chrome rather than content; fall back to ui.linenr, then
	// default.
	borderStyle := wig.Color("ui.linenr")
	if s, ok := wig.FindColor("comment"); ok {
		borderStyle = s
	}

	// Clear every cell we own (rows 0..vh-2) so stale cells from the
	// window below can never leak through.
	for y := 0; y < vh-1; y++ {
		view.SetContent(0, y, strings.Repeat(" ", vw), bg)
	}

	// Rounded box frame: rows 0 .. vh-3, cols 0 .. vw-1.
	boxTop := 0
	boxBottom := vh - 3
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

	// Content area sits inside the frame: one cell inset on each side, one
	// row inset top and bottom.
	contentX := 1
	contentW := vw - 2
	contentTop := 1
	contentH := boxBottom - contentTop // = vh - 4
	if contentW < 1 || contentH < 1 {
		return
	}

	hl, _ := u.buf.Highlighter.(*RgHighlighter)
	cur := wig.WindowCursorGet(u.e.ActiveWindow(), u.buf)
	if cur == nil {
		return
	}

	// Keep the cursor inside the visible viewport. The rg key handlers
	// (rgCursorDown, rgPageDown, ...) change cur.Line without touching
	// ScrollOffset, so doing it here — every frame — is what actually
	// scrolls the view when the cursor moves off-screen.
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

			// Cursor past the end of the line (EOL in normal mode).
			if lineNum == cur.Line && cur.Char >= len(runes)-1 && x < contentW {
				view.SetContent(contentX+x, y, " ", cursorStyle)
			}
		}
		line = line.Next()
		lineNum++
	}

	// Echo message row (vh-2). The statusline renderer writes here before
	// us and the rg key handlers use EchoMessage for their browse / replace
	// / search hints, so re-drawing it here (on top of the frame we just
	// painted) keeps the hints visible.
	msgRow := vh - 2
	view.SetContent(0, msgRow, strings.Repeat(" ", vw), bg)
	if msg := u.e.Message; msg != "" {
		msgStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		if s, ok := wig.FindColor("ui.message"); ok {
			msgStyle = s
		}
		view.SetContent(0, msgRow, msg, msgStyle)
	}
}
