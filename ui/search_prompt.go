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
	cursorPos   int
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
		cursorPos:   0,
		origCur:     *cur,
		origPattern: wig.LastSearchPattern,
	}

	cmdLine.keymap = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_INSERT: wig.KeyMap{
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
	wig.SelectionExtend(ctx.Buf, cur)
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
		switch ev.Key() {
		case tcell.KeyCtrlA:
			u.cursorPos = 0
		case tcell.KeyCtrlE:
			u.cursorPos = len(u.chBuf)
		case tcell.KeyCtrlB:
			if u.cursorPos > 0 {
				u.cursorPos--
			}
		case tcell.KeyCtrlF:
			if u.cursorPos < len(u.chBuf) {
				u.cursorPos++
			}
		case tcell.KeyCtrlU:
			u.chBuf = u.chBuf[u.cursorPos:]
			u.cursorPos = 0
			u.updateLiveSearch(ctx)
		case tcell.KeyCtrlK:
			u.chBuf = u.chBuf[:u.cursorPos]
			u.updateLiveSearch(ctx)
		case tcell.KeyCtrlW:
			if u.cursorPos == 0 {
				return
			}
			start := u.cursorPos
			for start > 0 && u.chBuf[start-1] == ' ' {
				start--
			}
			for start > 0 && u.chBuf[start-1] != ' ' {
				start--
			}
			u.chBuf = append(u.chBuf[:start], u.chBuf[u.cursorPos:]...)
			u.cursorPos = start
			u.updateLiveSearch(ctx)
		case tcell.KeyCtrlD:
			if u.cursorPos < len(u.chBuf) {
				u.chBuf = append(u.chBuf[:u.cursorPos], u.chBuf[u.cursorPos+1:]...)
				u.updateLiveSearch(ctx)
			}
		}
		return
	}
	if ev.Modifiers()&tcell.ModAlt != 0 {
		return
	}
	if ev.Modifiers()&tcell.ModMeta != 0 {
		return
	}
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if u.cursorPos > 0 {
			u.chBuf = append(u.chBuf[:u.cursorPos-1], u.chBuf[u.cursorPos:]...)
			u.cursorPos--
			u.updateLiveSearch(ctx)
		} else if len(u.chBuf) == 0 {
			u.cancel(ctx)
		}
		return
	case tcell.KeyDelete:
		if u.cursorPos < len(u.chBuf) {
			u.chBuf = append(u.chBuf[:u.cursorPos], u.chBuf[u.cursorPos+1:]...)
			u.updateLiveSearch(ctx)
		}
		return
	case tcell.KeyLeft:
		if u.cursorPos > 0 {
			u.cursorPos--
		}
		return
	case tcell.KeyRight:
		if u.cursorPos < len(u.chBuf) {
			u.cursorPos++
		}
		return
	case tcell.KeyHome:
		u.cursorPos = 0
		return
	case tcell.KeyEnd:
		u.cursorPos = len(u.chBuf)
		return
	case tcell.KeyEnter:
		cmd := strings.TrimSpace(string(u.chBuf))
		u.execute(ctx, cmd)
		return
	case tcell.KeyRune:
		u.chBuf = append(u.chBuf, 0)
		copy(u.chBuf[u.cursorPos+1:], u.chBuf[u.cursorPos:])
		u.chBuf[u.cursorPos] = ev.Rune()
		u.cursorPos++
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
		wig.SelectionExtend(ctx.Buf, cur)
	}
	u.e.Redraw()
}

func (u *uiSearchPrompt) execute(ctx wig.Context, cmd string) {
	pat := strings.TrimSpace(cmd)
	wig.LastSearchPattern = pat
	ctx.Editor.PopUiComponent(u)

	if len(pat) > 0 {
		cur := wig.ContextCursorGet(ctx)
		ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, &u.origCur)
		ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, cur)
	}
	u.e.Redraw()
}

func (u *uiSearchPrompt) Keymap() *wig.KeyHandler {
	return u.keymap
}

func (u *uiSearchPrompt) Render(view wig.View) {
	st := wig.Color("ui.statusline.powerline.normal")
	w, h := view.Size()
	h -= 2
	if h < 0 {
		return
	}
	bg := strings.Repeat(" ", w)
	view.SetContent(0, h, bg, st)
	before := string(u.chBuf[:u.cursorPos])
	atCursor := " "
	if u.cursorPos < len(u.chBuf) {
		atCursor = string(u.chBuf[u.cursorPos])
	}
	after := ""
	if u.cursorPos+1 < len(u.chBuf) {
		after = string(u.chBuf[u.cursorPos+1:])
	}
	promptPrefix := "/"
	view.SetContent(0, h, promptPrefix+before, st)
	cursorStyle := st.Reverse(true)
	view.SetContent(len([]rune(promptPrefix))+len([]rune(before)), h, atCursor, cursorStyle)
	if len(after) > 0 {
		view.SetContent(len([]rune(promptPrefix))+len([]rune(before))+1, h, after, st)
	}
	_ = fmt.Sprintf
}
func (u *uiSearchPrompt) Mode() wig.Mode {
	return wig.MODE_INSERT
}
