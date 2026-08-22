package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/firstrow/wig"
)

type MarksPopupWidget struct {
	e      *wig.Editor
	keymap *wig.KeyHandler
	items  []markItem
}

type markItem struct {
	Mark rune
	Line int
	Text string
}

func (u *MarksPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *MarksPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *MarksPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

func MarksPopupInit(ctx wig.Context, marks map[rune]wig.Cursor) {
	widget := &MarksPopupWidget{
		e: ctx.Editor,
	}

	var keys []rune
	for k := range marks {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		// Double backtick (pingpong): jump back and forward
		"`": func(ctx wig.Context) {
			ctx.Editor.PopUi()
			wig.CmdJumpToggle(ctx)
		},
	}

	for _, k := range keys {
		cur := marks[k]
		line := wig.CursorLineByNum(ctx.Buf, cur.Line)
		text := ""
		if line != nil {
			text = strings.TrimRight(line.Value.String(), "\n")
			text = strings.TrimSpace(text)
			if len(text) > 40 {
				text = text[:40] + "..."
			}
		}

		mark := k
		lineNum := cur.Line + 1
		widget.items = append(widget.items, markItem{
			Mark: mark,
			Line: lineNum,
			Text: text,
		})

		km[string(mark)] = func(ctx wig.Context) {
			ctx.Editor.PopUi()
			newCtx := ctx.Editor.NewContext()
			newCtx.Count = uint32(lineNum)
			wig.CmdGotoLine0(newCtx)
		}
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
}

func (u *MarksPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	// Build the list of lines to display
	lines := []string{}
	if len(u.items) == 0 {
		lines = append(lines, " No marks set. Press ` for pingpong, Esc to close. ")
	} else {
		for _, item := range u.items {
			s := fmt.Sprintf(" '%c' - Line %d: %s ", item.Mark, item.Line, item.Text)
			lines = append(lines, s)
		}
	}

	boxH := len(lines) + 1
	boxW := int(float32(vw) * 0.9)
	if boxW > vw {
		boxW = vw
	}

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := vh - boxH - 2

	// Use a dedicated popup/border style for the box, fallback to statusline
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
	view.SetContent(x+2, y, " Marks (` for pingpong) ", titleStyle)

	textStyle := wig.Color("ui.popup.text")
	if textStyle == wig.Color("default") {
		textStyle = wig.Color("ui.text")
	}

	markStyle := wig.Color("ui.mark")

	if len(u.items) == 0 {
		for i, line := range lines {
			view.SetContent(x+1, y+1+i, line, textStyle)
		}
	} else {
		for i, item := range u.items {
			cx := x + 1
			row := y + 1 + i

			prefix := " '"
			view.SetContent(cx, row, prefix, textStyle)
			cx += len([]rune(prefix))

			markChar := string(item.Mark)
			view.SetContent(cx, row, markChar, markStyle)
			cx += 1

			rest := fmt.Sprintf("' - Line %d: %s ", item.Line, item.Text)
			view.SetContent(cx, row, rest, textStyle)
		}
	}
}
