package ui

import (
	"github.com/firstrow/wig"
)

func NotificationsRender(e *wig.Editor, view wig.View) {
	vw, vh := view.Size()

	x := vw - 53
	y := vh - 5
	w := 50
	h := 3

	drawBox(view, x, y, w, h, wig.Color("ui.statusline"))
	view.SetContent(x+1, y+1, truncate("sdf", 48), wig.Color("ui.statusline"))
}
