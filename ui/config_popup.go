package ui

import (
	"fmt"

	"github.com/firstrow/wig"
)

type configItem struct {
	Name    string
	Value   interface{}
	Options []string
}

type ConfigPopupWidget struct {
	e            *wig.Editor
	keymap       *wig.KeyHandler
	items        []configItem
	active       int
	scrollOffset int
}

func (u *ConfigPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *ConfigPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *ConfigPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

func ConfigPopupInit(ctx wig.Context) {
	cfg := &ctx.Editor.Config

	widget := &ConfigPopupWidget{
		e:      ctx.Editor,
		active: 0,
		items: []configItem{
			{Name: "show_line_numbers", Value: &cfg.ShowLineNumbers},
			{Name: "relative_line_numbers", Value: &cfg.RelativeLineNumbers},
			{Name: "current_line_absolute", Value: &cfg.CurrentLineAbsolute},
			{Name: "format_on_save", Value: &cfg.FormatOnSave},
			{Name: "indent_guides", Value: &cfg.IndentGuides},
			{Name: "lsp_enabled", Value: &cfg.LspEnabled},
			{Name: "same_statusline_color", Value: &cfg.SameStatuslineColor},
			{Name: "save_workspaces", Value: &cfg.SaveWorkspaces},
			{Name: "comment_style", Value: &cfg.CommentStyle, Options: []string{"standard", "simple"}},
			{Name: "which_key_format", Value: &cfg.WhichKeyFormat, Options: []string{"words", "cmd", "camelcase"}},
			{Name: "git_status_view", Value: &cfg.GitStatusView, Options: []string{"full", "split"}},
			{Name: "git_blame_view", Value: &cfg.GitBlameView, Options: []string{"split", "full"}},
			{Name: "quickfix_view", Value: &cfg.QuickfixView, Options: []string{"split", "popup"}},
		},
	}

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"q": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"j": func(ctx wig.Context) {
			widget.moveDown(1)
		},
		"Down": func(ctx wig.Context) {
			widget.moveDown(1)
		},
		"k": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Up": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Space": func(ctx wig.Context) {
			widget.toggleValue()
		},
		"Enter": func(ctx wig.Context) {
			widget.toggleValue()
		},
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
}

func (u *ConfigPopupWidget) moveDown(n int) {
	u.active = min(u.active+n, len(u.items)-1)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *ConfigPopupWidget) moveUp(n int) {
	u.active = max(u.active-n, 0)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *ConfigPopupWidget) ensureVisible() {
	maxVis := 10
	if u.active < u.scrollOffset {
		u.scrollOffset = u.active
	} else if u.active >= u.scrollOffset+maxVis {
		u.scrollOffset = u.active - maxVis + 1
	}
}

func (u *ConfigPopupWidget) toggleValue() {
	item := &u.items[u.active]
	switch v := item.Value.(type) {
	case *bool:
		*v = !*v
	case *string:
		if len(item.Options) > 0 {
			currentIdx := 0
			for i, opt := range item.Options {
				if opt == *v {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(item.Options)
			*v = item.Options[nextIdx]
		}
	}
	u.e.Redraw()
}

func (u *ConfigPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	boxW := int(float32(vw) * 0.9)
	if boxW > vw {
		boxW = vw
	}
	boxH := int(float32(vh) * 0.9)
	if boxH > vh {
		boxH = vh
	}

	maxItems := boxH - 4
	if maxItems < 1 {
		maxItems = 1
	}
	visCount := len(u.items)
	if visCount > maxItems {
		visCount = maxItems
	}

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (vh - boxH) / 2
	if y < 0 {
		y = 0
	}

	style := wig.Color("default")
	drawBox(view, x, y, boxW, boxH, style)
	view.SetContent(x+2, y, " Config ", wig.Color("ui.popup.title"))

	endIdx := min(u.scrollOffset+visCount, len(u.items))
	for i, item := range u.items[u.scrollOffset:endIdx] {
		row := y + 2 + i
		if row >= y+boxH-1 {
			break
		}

		cursorPrefix := "  "
		if u.scrollOffset+i == u.active {
			cursorPrefix = "> "
		}
		view.SetContent(x+1, row, cursorPrefix, style)

		nameStyle := wig.Color("default")
		if u.scrollOffset+i == u.active {
			nameStyle = wig.Color("ui.menu.selected")
		}
		view.SetContent(x+3, row, item.Name, nameStyle)

		valStr := ""
		switch v := item.Value.(type) {
		case *bool:
			valStr = fmt.Sprintf("%v", *v)
		case *string:
			valStr = *v
		}

		valLen := len(valStr)
		valX := x + boxW - 2 - valLen
		view.SetContent(valX, row, valStr, nameStyle)
	}

	view.SetContent(x+2, y+boxH-1, " [Space/Enter] toggle/cycle [Esc/q] close ", wig.Color("ui.linenr"))
}
