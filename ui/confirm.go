package ui

import (
	"strings"

	"github.com/firstrow/wig"
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
	y := vh - 2
	if y < 0 {
		return
	}

	st := wig.Color("default")
	if s, ok := wig.FindColor("ui.message"); ok {
		st = s
	}

	// Fill the message line with the default background
	bg := strings.Repeat(" ", vw)
	view.SetContent(0, y, bg, wig.Color("default"))

	// Render the prompt text starting at x = 0
	view.SetContent(0, y, u.prompt, st)

	// Render a cursor block at the end of the prompt
	cursorStyle := wig.Color("ui.cursor")
	if len(u.prompt) < vw {
		view.SetContent(len(u.prompt), y, " ", cursorStyle)
	}
}
