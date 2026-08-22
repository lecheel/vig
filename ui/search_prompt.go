package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

type uiSearchPrompt struct {
	e           *wig.Editor
	keymap      *wig.KeyHandler
	chBuf       []rune
	origCur     wig.Cursor
	origPattern string
}

func (u *uiSearchPrompt) Plane() wig.RenderPlane {
	return wig.PlaneEditor
}

func CmdSearchPromptInit(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	cmdLine := &uiSearchPrompt{
		e:           ctx.Editor,
		chBuf:       []rune{},
		origCur:     *cur,
		origPattern: wig.LastSearchPattern,
	}

	cmdLine.keymap = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Esc": func(c wig.Context) {
				cmdLine.cancel(c)
			},
			"ctrl+c": func(c wig.Context) {
				cmdLine.cancel(c)
			},
		},
	})
	cmdLine.keymap.Fallback(cmdLine.insertCh)
	ctx.Editor.PushUi(cmdLine)
	ctx.Editor.Redraw()
}

func (u *uiSearchPrompt) cancel(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	*cur = u.origCur
	wig.LastSearchPattern = u.origPattern
	ctx.Editor.PopUiComponent(u)
	wig.CmdEnsureCursorVisible(ctx)
	ctx.Editor.Redraw()
}

func (u *uiSearchPrompt) insertCh(ctx wig.Context, ev *tcell.EventKey) {
	if ev.Key() == tcell.KeyEsc || ev.Key() == tcell.KeyCtrlC {
		u.cancel(ctx)
		return
	}

	if ev.Modifiers()&tcell.ModCtrl != 0 {
		return
	}

	if ev.Modifiers()&tcell.ModAlt != 0 {
		return
	}

	if ev.Modifiers()&tcell.ModMeta != 0 {
		return
	}

	if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
		if len(u.chBuf) > 0 {
			u.chBuf = u.chBuf[:len(u.chBuf)-1]
			u.updateLiveSearch(ctx)
		} else {
			u.cancel(ctx)
		}
		return
	}

	if ev.Key() == tcell.KeyEnter {
		cmd := strings.TrimSpace(string(u.chBuf))
		u.execute(ctx, cmd)
		return
	}

	if ev.Key() == tcell.KeyRune {
		u.chBuf = append(u.chBuf, ev.Rune())
		u.updateLiveSearch(ctx)
	}
}

func (u *uiSearchPrompt) updateLiveSearch(ctx wig.Context) {
	pat := string(u.chBuf)
	wig.LastSearchPattern = pat
	if len(pat) > 0 {
		wig.SearchFrom(ctx, u.origCur, pat)
	} else {
		cur := wig.ContextCursorGet(ctx)
		*cur = u.origCur
		wig.CmdEnsureCursorVisible(ctx)
	}
	u.e.Redraw()
}

func (u *uiSearchPrompt) execute(ctx wig.Context, cmd string) {
	pat := strings.TrimSpace(cmd)
	wig.LastSearchPattern = pat
	ctx.Editor.PopUiComponent(u)

	if len(pat) > 0 {
		cur := wig.ContextCursorGet(ctx)
		ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, cur)
	}
	u.e.Redraw()
}

func (u *uiSearchPrompt) Keymap() *wig.KeyHandler {
	return u.keymap
}

func (u *uiSearchPrompt) Render(view wig.View) {
	st := wig.Color("statusline")
	w, h := view.Size()
	h -= 1

	bg := strings.Repeat(" ", w)
	view.SetContent(0, h, bg, st)

	msg := fmt.Sprintf("/%s%s", string(u.chBuf), string(tcell.RuneBlock))
	view.SetContent(0, h, msg, st)
}

func (u *uiSearchPrompt) Mode() wig.Mode {
	return wig.MODE_NORMAL
}
