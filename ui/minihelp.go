package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
)

type MiniHelpWidget struct {
	e      *wig.Editor
	keymap *wig.KeyHandler
	items  []string
}

func (u *MiniHelpWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *MiniHelpWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *MiniHelpWidget) Keymap() *wig.KeyHandler { return u.keymap }

func MiniHelpInit(ctx wig.Context) *MiniHelpWidget {
	w := &MiniHelpWidget{e: ctx.Editor}

	keys := []string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"}
	keyDescs := ctx.Editor.Keys.GetActionDescriptions(keys)

	for _, k := range keys {
		if desc, ok := keyDescs[k]; ok {
			w.items = append(w.items, fmt.Sprintf("%s %s", k, desc))
		} else {
			w.items = append(w.items, fmt.Sprintf("%s -", k))
		}
	}

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"q": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"Enter": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
	}
	w.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(w)
	return w
}

func CmdMiniHelp(ctx wig.Context) {
	MiniHelpInit(ctx)
}

func (u *MiniHelpWidget) Render(view wig.View) {
	vw, vh := view.Size()

	boxW := vw
	if boxW > 140 {
		boxW = 140
	}
	colW := (boxW - 2) / 6 // 6 per row
	boxH := 3              // 2 rows of items (drawBox2 adds the top/bottom borders itself) dont know why need 3 for 2 lines

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := vh - boxH - 2 // Move 1 line up

	// Use a dedicated popup/border style for the box, fallback to statusline, then linenr
	style := wig.Color("ui.popup.border")
	if style == wig.Color("default") {
		style = wig.Color("ui.statusline")
	}
	if style == wig.Color("default") {
		style = wig.Color("ui.linenr")
	}

	drawBox2(view, x, y, boxW, boxH, style)

	// Title on top border
	titleStyle := wig.Color("ui.popup.title")
	if titleStyle == wig.Color("default") {
		titleStyle = style
	}
	view.SetContent(x+2, y, " F - Keys ", titleStyle)

	textStyle := wig.Color("ui.popup.text")
	if textStyle == wig.Color("default") {
		textStyle = wig.Color("ui.text")
	}
	if textStyle == wig.Color("default") {
		textStyle = style.Reverse(true)
	}

	// Row 1: F1 to F6
	// Row 2: F7 to F12
	var row1, row2 strings.Builder
	for i := 0; i < 6; i++ {
		row1.WriteString(fmt.Sprintf("%-*s", colW, truncate(u.items[i], colW)))
	}
	for i := 6; i < 12; i++ {
		row2.WriteString(fmt.Sprintf("%-*s", colW, truncate(u.items[i], colW)))
	}

	view.SetContent(x+1, y+1, row1.String(), textStyle)
	view.SetContent(x+1, y+2, row2.String(), textStyle)
}
