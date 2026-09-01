package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func StatuslineRender(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
) {
	buf := win.Buffer()
	if buf == nil {
		return
	}
	w, h := view.Size()
	h -= 1
	if h < 0 {
		return
	}

	stActive := wig.Color("ui.statusline")
	stInactive := wig.Color("ui.statusline.inactive")
	stInsert := wig.Color("ui.statusline.insert")

	st := stInactive
	if win == e.ActiveWindow() {
		st = stActive
		if !e.Config.SameStatuslineColor && buf.Mode() == wig.MODE_INSERT {
			st = stInsert
		}
	}

	// Background fill
	bgFill := strings.Repeat(" ", w)
	view.SetContent(0, h, bgFill, st)

	type segment struct {
		text  string
		style tcell.Style
	}

	stContrast := st.Reverse(true)

	var leftSegs []segment
	var rightSegs []segment

	modeText := fmt.Sprintf(" %s ", strings.ToUpper(buf.Mode().String()))
	leftSegs = append(leftSegs, segment{text: modeText, style: st})

	nameText := fmt.Sprintf(" %s ", buf.GetName())
	leftSegs = append(leftSegs, segment{text: nameText, style: stContrast})

	macroStatus := ""
	if e.Keys.Macros.Recording() {
		macroStatus = fmt.Sprintf(" REC @%s ", e.Keys.Macros.Register)
		leftSegs = append(leftSegs, segment{text: macroStatus, style: st})
	}

	if (win == e.ActiveWindow() || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		leftSegs = []segment{{text: fmt.Sprintf(" %s ", e.Message), style: st}}
	}

	cur := wig.CursorGet(e, buf)

	wsIndicator := "ð"
	if e.Config.SaveWorkspaces {
		wsIndicator = "ð"
	}
	wsText := fmt.Sprintf(" %s ws:%d ", wsIndicator, e.ActiveWorkspace)
	rightSegs = append(rightSegs, segment{text: wsText, style: stContrast})

	posText := fmt.Sprintf(" %d:%d ", cur.Line+1, cur.Char)
	rightSegs = append(rightSegs, segment{text: posText, style: st})

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}
	if funcName != "" {
		if len(funcName) > 30 {
			funcName = funcName[:27] + "..."
		}
		rightSegs = append([]segment{{text: fmt.Sprintf(" %s ", funcName), style: stContrast}}, rightSegs...)
	}

	if e.Keys.GetCount() > 1 {
		rightSegs = append([]segment{{text: fmt.Sprintf(" %d ", e.Keys.GetCount()), style: st}}, rightSegs...)
	}

	baseBg := wig.GetStyleBg("ui.statusline")
	if win != e.ActiveWindow() {
		baseBg = wig.GetStyleBg("ui.statusline.inactive")
	}

	arrowL := "\ue0b0"
	arrowR := "\ue0b2"
	aw := runewidth.StringWidth(arrowL)
	if aw == 0 {
		aw = 1
	}

	// Draw left side
	x := 0
	prevBg := baseBg
	for _, s := range leftSegs {
		_, bg, _ := s.style.Decompose()
		if bg == tcell.ColorDefault {
			bg = baseBg
		}
		arrowStyle := tcell.StyleDefault.Background(bg).Foreground(prevBg)
		view.SetContent(x, h, arrowL, arrowStyle)
		x += aw

		view.SetContent(x, h, s.text, s.style)
		x += runewidth.StringWidth(s.text)
		prevBg = bg
	}

	// Draw right side
	x = w
	prevBg = baseBg
	for i := len(rightSegs) - 1; i >= 0; i-- {
		s := rightSegs[i]
		_, bg, _ := s.style.Decompose()
		if bg == tcell.ColorDefault {
			bg = baseBg
		}
		textWidth := runewidth.StringWidth(s.text)

		// Draw text
		view.SetContent(x-textWidth, h, s.text, s.style)
		x -= textWidth

		// Draw arrow on the left of the text
		arrowStyle := tcell.StyleDefault.Background(prevBg).Foreground(bg)
		view.SetContent(x-aw, h, arrowR, arrowStyle)
		x -= aw
		prevBg = bg
	}
}
