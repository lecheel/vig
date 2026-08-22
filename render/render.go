package render

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	_ "github.com/gdamore/tcell/v2/encoding"
	"github.com/gdamore/tcell/v2/views"
	"github.com/mattn/go-runewidth"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

type Renderer struct {
	rw      sync.Mutex
	e       *wig.Editor
	screen  tcell.Screen
	stopped bool
}

func New(e *wig.Editor, screen tcell.Screen) *Renderer {
	r := &Renderer{
		e:      e,
		screen: screen,
	}
	return r
}

// Stop prevents further rendering. This must be called before tscreen.Fini()
// to avoid a data race / panic when the input loop tries to render one last
// time after the screen has been finalized.
func (r *Renderer) Stop() {
	r.rw.Lock()
	defer r.rw.Unlock()
	r.stopped = true
}

// TODO: rendering must be optimized.
func (r *Renderer) Render() {
	// TODO: schedule render
	r.rw.Lock()
	defer r.rw.Unlock()

	if r.stopped {
		return
	}

	r.screen.Fill(' ', wig.Color("ui.background"))

	w, h := r.screen.Size()

	var winW int
	if r.e.Layout == wig.LayoutVertical {
		winW = w / len(r.e.Windows)
	} else {
		winW = w
	}

	var winView *mview
	var activeWinView *mview
	for i, win := range r.e.Windows {
		x := winW * i
		if i > 0 {
			st := wig.Color("ui.virtual.indent-guide")
			for i := 0; i <= h; i++ {
				if x >= 0 && x < w {
					r.SetContent(x, i, string(tcell.RuneVLine), st)
				}
			}
			x += 1
		}

		if winW <= 0 || h <= 0 {
			continue
		}

		winView = NewMView(r.screen, x, 0, winW, h)

		if win == r.e.ActiveWindow() {
			activeWinView = winView
		}

		ui.WindowRender(r.e, winView, win)

		// Draw indent guides overlay
		if r.e.Config.IndentGuides {
			r.RenderIndentGuides(win, winView)
		}

		ui.StatuslineRender(r.e, winView, win)
	}

	// widgets: pickers, etc...
	mainView := NewMView(r.screen, 0, 0, w, h)
	for _, c := range r.e.UiComponents {
		switch c.Plane() {
		case wig.PlaneWin:
			c.Render(activeWinView)
		default:
			c.Render(mainView)
		}
	}

	r.screen.Show()
}

// RenderIndentGuides draws vertical guide lines over leading whitespace
func (r *Renderer) RenderIndentGuides(win *wig.Window, view *mview) {
	buf := win.Buffer()
	if buf == nil {
		return
	}

	cur := wig.WindowCursorGet(win, buf)
	if cur == nil {
		return
	}

	viewW, viewH := view.Size()
	// Calculate the X offset where text begins. This MUST mirror the
	// leftPadding calculation in ui.WindowRender exactly (sign column,
	// line-number width, blame column) or guides land on top of real
	// text/columns instead of staying inside the leading whitespace.
	// Using the shared ui.WindowTextPadding helper guarantees the two
	// renderers stay in sync; the previous duplicate calculation used
	// len(buf.GitSigns) > 0 while WindowRender used true, so textX was
	// 2 cells too small when a buffer had no git signs, and guides were
	// drawn over the line-number gutter instead of in the leading
	// whitespace.
	textX := ui.WindowTextPadding(r.e, buf)
	// Reuse the split style for indent guides if it exists,
	// otherwise fallback to a default style
	style := wig.Color("ui.virtual.indent-guide")
	if style == tcell.StyleDefault {
		style = wig.Color("ui.indentguide")
	}
	cursorLineStyle := wig.ApplyBg("ui.cursorline", style)
	scrollX := 0 // Horizontal scroll is not explicitly tracked here, assuming 0
	lineNum := cur.ScrollOffset
	line := wig.CursorLineByNum(buf, lineNum)

	// Precompute visual-block selection column bounds (if any) so indent
	// guides don't render on top of the selection highlight in visual
	// block mode, mirroring the same calculation in ui.WindowRender.
	var selMinLine, selMaxLine, selMinVisCol, selMaxVisCol int
	isVisualBlockSel := buf.Selection != nil && buf.Mode() == wig.MODE_VISUAL_BLOCK
	if isVisualBlockSel {
		sel := buf.Selection
		startLineNode := wig.CursorLineByNum(buf, sel.Start.Line)
		endLineNode := wig.CursorLineByNum(buf, sel.End.Line)
		if startLineNode != nil && endLineNode != nil {
			startVisCol := wig.VisualCol(startLineNode.Value, sel.Start.Char)
			endVisCol := wig.VisualCol(endLineNode.Value, sel.End.Char)
			selMinVisCol = min(startVisCol, endVisCol)
			selMaxVisCol = max(startVisCol, endVisCol)
			selMinLine = min(sel.Start.Line, sel.End.Line)
			selMaxLine = max(sel.Start.Line, sel.End.Line)
		} else {
			isVisualBlockSel = false
		}
	}
	// Character-wise (Visual / Visual Line) selections: tested per-cell via
	// SelectionCursorInRange, same as ui.WindowRender does for text glyphs.
	hasCharSel := buf.Selection != nil && (buf.Mode() == wig.MODE_VISUAL || buf.Mode() == wig.MODE_VISUAL_LINE)

	for y := 0; y < viewH && line != nil; y++ {
		lineRun := line.Value
		// Blank lines have no leading whitespace of their own, which would
		// otherwise break the vertical guide as it passes through a block.
		// Borrow indentation from the next non-blank line so the guide
		// stays continuous, matching typical indent-guide behavior.
		if isBlankLine(lineRun) {
			next := line.Next()
			for next != nil && isBlankLine(next.Value) {
				next = next.Next()
			}
			if next == nil {
				line = line.Next()
				lineNum++
				continue
			}
			lineRun = next.Value
		}
		lineStyle := style
		isCursorLine := lineNum == cur.Line
		if isCursorLine {
			lineStyle = cursorLineStyle
		}
		// cur.Char is a rune index, but guide positions are visual (tab-
		// expanded) columns — comparing them directly is comparing two
		// different scales and causes guides at higher rune indices to be
		// incorrectly skipped as "under the cursor". Convert once per row.
		cursorVisCol := -1
		if isCursorLine {
			cursorVisCol = wig.VisualCol(line.Value, cur.Char)
		}
		for _, pos := range wig.IndentGuideColumns(lineRun) {
			relX := pos - scrollX
			if relX < 0 || relX >= viewW {
				continue
			}
			// Never draw over the actual cursor cell.
			if isCursorLine && pos == cursorVisCol {
				continue
			}
			// Never draw over an active selection highlight — otherwise the
			// guide glyph paints its own (unselected) background on top of
			// the selection color, making it look like the selection has a
			// gap wherever an indent guide falls (e.g. pressing 'v' + 'j').
			if isVisualBlockSel && lineNum >= selMinLine && lineNum <= selMaxLine && pos >= selMinVisCol && pos < selMaxVisCol {
				continue
			}
			if hasCharSel && wig.SelectionCursorInRange(buf.Selection, wig.Cursor{Line: lineNum, Char: pos}) {
				continue
			}
			screenX := textX + relX
			view.SetContent(screenX, y, wig.IndentGuideGlyph, lineStyle)
		}
		line = line.Next()
		lineNum++
	}
}

// isBlankLine reports whether a line contains only whitespace (i.e. no
// visible content besides the trailing newline).
func isBlankLine(lineRun []rune) bool {
	for _, r := range lineRun {
		if r != ' ' && r != '\t' && r != '\n' {
			return false
		}
	}
	return true
}
func (r *Renderer) SetContent(x, y int, str string, st tcell.Style) {
	for _, ch := range str {
		var comb []rune
		w := runewidth.RuneWidth(ch)
		if w == 0 {
			comb = []rune{ch}
			ch = ' '
			w = 1
		}

		r.screen.SetContent(x, y, ch, comb, st)
		x += w
	}
}

func (r *Renderer) RenderMetrics(info map[string]time.Duration) {
	y := 0
	for k, v := range info {
		r.SetContent(50, y, fmt.Sprintf("%s: %v", k, v), tcell.StyleDefault)
		y++
	}
}

type mview struct {
	viewport *views.ViewPort
}

func NewMView(view views.View, x, y, w, h int) *mview {
	return &mview{
		viewport: views.NewViewPort(view, x, y, w, h),
	}
}

func (t *mview) Size() (int, int) {
	return t.viewport.Size()
}

func (t *mview) Resize(x, y, width, height int) {
	t.viewport.Resize(x, y, width, height)
}

func (t *mview) SetContent(x, y int, str string, st tcell.Style) {
	for _, ch := range str {
		var comb []rune
		w := runewidth.RuneWidth(ch)
		if w == 0 {
			comb = []rune{ch}
			ch = ' '
			w = 1
		}

		t.viewport.SetContent(x, y, ch, comb, st)
		x += w
	}
}
