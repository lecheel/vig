package ui

import (
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

type ConfirmWidget struct {
	editor   *wig.Editor
	keymap   *wig.KeyHandler
	prompt   string
	onYes    func()
	onNo     func()
	onCancel func()
}

func (u *ConfirmWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *ConfirmWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *ConfirmWidget) Keymap() *wig.KeyHandler { return u.keymap }

func ConfirmInit(ctx wig.Context, prompt string, onYes func(), onNo func(), onCancel func()) *ConfirmWidget {
	widget := &ConfirmWidget{
		editor:   ctx.Editor,
		prompt:   prompt,
		onYes:    onYes,
		onNo:     onNo,
		onCancel: onCancel,
	}

	km := wig.KeyMap{
		"y": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onYes != nil {
				widget.onYes()
			}
		},
		"Y": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onYes != nil {
				widget.onYes()
			}
		},
		"Enter": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onYes != nil {
				widget.onYes()
			}
		},
		"n": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onNo != nil {
				widget.onNo()
			}
		},
		"N": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onNo != nil {
				widget.onNo()
			}
		},
		"c": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onCancel != nil {
				widget.onCancel()
			}
		},
		"C": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onCancel != nil {
				widget.onCancel()
			}
		},
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			if widget.onCancel != nil {
				widget.onCancel()
			}
		},
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
	return widget
}

func (u *ConfirmWidget) Render(view wig.View) {
	vw, vh := view.Size()
	y := vh - 1

	// Use the statusline style to blend in with the bottom bar
	st := wig.Color("ui.statusline")

	// Fill the entire bottom line with the background color
	bg := strings.Repeat(" ", vw)
	view.SetContent(0, y, bg, st)

	// Render the prompt text
	view.SetContent(0, y, u.prompt, st)

	// Render a red cursor block at the end of the prompt
	cursorStyle := tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorWhite)
	if len(u.prompt) < vw {
		view.SetContent(len(u.prompt), y, " ", cursorStyle)
	}
}
