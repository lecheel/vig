package ui

import (
	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		maxLen = 3
	}
	return string(runes[0:maxLen-3]) + "..."
}

func drawBox(s wig.View, x, y, width, height int, style tcell.Style) {
	if width <= 0 || height <= 0 {
		return
	}

	x1, y1 := x, y
	x2, y2 := x+width-1, y+height-1

	for col := x1; col <= x2; col++ {
		s.SetContent(col, y1, string(tcell.RuneHLine), style)
		s.SetContent(col, y2, string(tcell.RuneHLine), style)
	}
	for row := y1 + 1; row < y2; row++ {
		s.SetContent(x1, row, string(tcell.RuneVLine), style)
		s.SetContent(x2, row, string(tcell.RuneVLine), style)
	}
	if y1 != y2 && x1 != x2 {
		s.SetContent(x1, y1, "╭", style)
		s.SetContent(x2, y1, "╮", style)
		s.SetContent(x1, y2, "╰", style)
		s.SetContent(x2, y2, "╯", style)
	}

	// fill bg
	for row := y1 + 1; row < y2; row++ {
		for col := x1 + 1; col < x2; col++ {
			s.SetContent(col, row, " ", style)
		}
	}
}

func drawBoxNoBorder(s wig.View, x, y, width, height int, style tcell.Style) {
	if width <= 0 || height <= 0 {
		return
	}

	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			s.SetContent(col, row, " ", style)
		}
	}
}
