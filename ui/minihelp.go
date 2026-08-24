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
	boxH := 4              // 2 rows of items + 2 border lines

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := vh - boxH - 1

	// Use default colors to match the marks popup exactly
	style := wig.Color("default")

	drawBox(view, x, y, boxW, boxH, style)

	// Title on top border
	view.SetContent(x+2, y, " F - Keys ", wig.Color("ui.popup.title"))

	textStyle := wig.Color("default")

	// Row 1: F1 to F6
	// Row 2: F7 to F12
	shortcutStyle := wig.Color("ui.whichkey.key")
	keyWidth := 3 // Length of "F12"

	renderRow := func(rowItems []string, yPos int) {
		cx := x + 1
		for _, item := range rowItems {
			parts := strings.SplitN(item, " ", 2)
			keyStr := parts[0]
			descStr := ""
			if len(parts) > 1 {
				descStr = parts[1]
			}

			descMaxW := colW - keyWidth - 1
			if descMaxW < 0 {
				descMaxW = 0
			}
			descRender := truncate(descStr, descMaxW)

			// Render key padded to keyWidth
			view.SetContent(cx, yPos, fmt.Sprintf("%-*s", keyWidth, keyStr), shortcutStyle)
			// Render description padded to descMaxW
			view.SetContent(cx+keyWidth+1, yPos, fmt.Sprintf("%-*s", descMaxW, descRender), textStyle)

			cx += colW
		}
	}

	renderRow(u.items[0:6], y+1)
	renderRow(u.items[6:12], y+2)
}
