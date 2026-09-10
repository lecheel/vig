package ui

import (
	"fmt"
	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"strings"
)

type uiSearchPrompt struct {
	e      *wig.Editor
	keymap *wig.KeyHandler
	*LineEditor
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
		LineEditor:  NewLineEditor(""),
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
	if ev.Key() == tcell.KeyEnter {
		u.execute(ctx, strings.TrimSpace(u.Text()))
		return
	}
	// Backspace on an empty prompt cancels the search.
	if (ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2) && len(u.chBuf) == 0 {
		u.cancel(ctx)
		return
	}
	handled, changed := u.HandleReadlineKey(ev)
	if !handled {
		return
	}
	if changed {
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
