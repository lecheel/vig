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

// drawBox is a thin wrapper around wig.DrawBox so existing callers in
// the ui package keep working. The real implementation lives in package
// wig (see draw.go) so it can be shared with code that lives in package
// wig itself (e.g. the WhichKey popup) without an import cycle.
func drawBox(s wig.View, x1, y1, x2, y2 int, style tcell.Style) {
	wig.DrawBox(s, x1, y1, x2, y2, style)
}

func drawBox2(s wig.View, x, y, width, height int, style tcell.Style) {
	drawBox(s, x, y, x+width, y+height, style)
}

func drawBoxNoBorder(s wig.View, x1, y1, width, height int, style tcell.Style) {
	x2 := x1 + width
	y2 := y1 + height
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	if x2 < x1 {
		x1, x2 = x2, x1
	}

	for row := y1; row < y2; row++ {
		for col := x1; col < x2; col++ {
			s.SetContent(col, row, " ", style)
		}
	}
}
