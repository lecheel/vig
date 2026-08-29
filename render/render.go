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
	rw        sync.Mutex
	e         *wig.Editor
	screen    tcell.Screen
	stopped   bool
	suspended bool
}

func New(e *wig.Editor, screen tcell.Screen) *Renderer {
	r := &Renderer{
		e:      e,
		screen: screen,
	}
	if mv, ok := e.View.(*mview); ok {
		mv.renderer = r
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

// Suspend releases the terminal (leaves the alternate screen, restores the
// original terminal mode) so an external interactive program can take over
// stdin/stdout/stderr. Pairs with Resume. Implements commands.Suspendable.
func (r *Renderer) Suspend() error {
	r.rw.Lock()
	r.suspended = true
	r.rw.Unlock()
	return r.screen.Suspend()
}

// Resume re-acquires the terminal after a prior Suspend, re-entering the
// alternate screen and restoring tcell's raw input mode. The caller should
// follow this with a full Redraw + ScreenSync since the terminal's actual
// contents were overwritten by whatever ran during the suspend.
func (r *Renderer) Resume() error {
	r.rw.Lock()
	defer r.rw.Unlock()
	err := r.screen.Resume()
	r.suspended = false
	return err
}

// TODO: rendering must be optimized.
func (r *Renderer) Render() {
	// TODO: schedule render
	r.rw.Lock()
	defer r.rw.Unlock()

	if r.stopped || r.suspended {
		return
	}

	r.screen.Fill(' ', wig.Color("ui.background"))

	w, h := r.screen.Size()

	ws := r.e.GetActiveWorkspace()
	var root *wig.WinNode
	if ws != nil {
		root = ws.Root
	}
	if root == nil {
		if aw := r.e.ActiveWindow(); aw != nil {
			root = wig.LeafNode(aw)
		}
	}

	var activeWinView *mview
	if root != nil {
		activeWinView = r.renderNode(root, 0, 0, w, h)
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

func (r *Renderer) renderNode(node *wig.WinNode, x, y, w, h int) *mview {
	if node == nil || w <= 0 || h <= 0 {
		return nil
	}
	if node.Dir == wig.SplitNone {
		if node.Win == nil {
			return nil
		}
		view := NewMView(r.screen, x, y, w, h)
		ui.WindowRender(r.e, view, node.Win)
		if r.e.Config.IndentGuides {
			r.RenderIndentGuides(node.Win, view)
		}
		ui.StatuslineRender(r.e, view, node.Win)
		if node.Win == r.e.ActiveWindow() {
			return view
		}
		return nil
	}
	n := len(node.Children)
	if n == 0 {
		return nil
	}
	var active *mview
	st := wig.Color("ui.virtual.indent-guide")
	if node.Dir == wig.SplitVertical {
		numSeps := n - 1
		availW := max(w-numSeps, 0)
		baseW := availW / n
		remW := availW % n

		curX := x
		for i, c := range node.Children {
			cw := baseW
			if i < remW {
				cw++
			}
			if i > 0 {
				sepX := curX - 1
				for j := 0; j < h; j++ {
					r.SetContent(sepX, y+j, string(tcell.RuneVLine), st)
				}
			}
			if v := r.renderNode(c, curX, y, cw, h); v != nil {
				active = v
			}
			curX += cw + 1
		}
	} else {
		numSeps := n - 1
		availH := max(h-numSeps, 0)
		baseH := availH / n
		remH := availH % n

		curY := y
		for i, c := range node.Children {
			ch := baseH
			if i < remH {
				ch++
			}
			if i > 0 {
				sepY := curY - 1
				for j := 0; j < w; j++ {
					r.SetContent(x+j, sepY, string(tcell.RuneHLine), st)
				}
			}
			if v := r.renderNode(c, x, curY, w, ch); v != nil {
				active = v
			}
			curY += ch + 1
		}
	}
	return active
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
	view     views.View
	renderer *Renderer
}

func NewMView(view views.View, x, y, w, h int) *mview {
	return &mview{
		viewport: views.NewViewPort(view, x, y, w, h),
		view:     view,
	}
}

// Suspend releases the terminal (leaves the alternate screen, restores the
// original terminal mode) so an external interactive program can take over
// stdin/stdout/stderr. Pairs with Resume. Implements commands.Suspendable.
// This delegates to the underlying tcell.Screen, which handles x/term state
// restoration and stops its internal input loop from stealing keystrokes.
func (m *mview) Suspend() error {
	if m.renderer != nil {
		m.renderer.rw.Lock()
		m.renderer.suspended = true
		m.renderer.rw.Unlock()
	}
	if s, ok := m.view.(tcell.Screen); ok {
		return s.Suspend()
	}
	return fmt.Errorf("view does not support suspend")
}

// Resume re-acquires the terminal after a prior Suspend, re-entering the
// alternate screen and restoring tcell's raw input mode. The caller should
// follow this with a full Redraw + ScreenSync since the terminal's actual
// contents were overwritten by whatever ran during the suspend.
func (m *mview) Resume() error {
	if m.renderer != nil {
		m.renderer.rw.Lock()
		defer m.renderer.rw.Unlock()
	}
	if s, ok := m.view.(tcell.Screen); ok {
		err := s.Resume()
		if m.renderer != nil {
			m.renderer.suspended = false
		}
		return err
	}
	return fmt.Errorf("view does not support resume")
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
