package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/firstrow/wig"
)

type RegistersPopupWidget struct {
	e      *wig.Editor
	keymap *wig.KeyHandler
	items  []registerItem
}

type registerItem struct {
	Key     rune
	Preview string
}

func (u *RegistersPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *RegistersPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *RegistersPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

func firstNonEmptyLine(text string) string {
	for _, l := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func RegistersPopupInit(ctx wig.Context) {
	widget := &RegistersPopupWidget{
		e: ctx.Editor,
	}

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"q": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
	}

	addItem := func(key rune, content string) {
		preview := firstNonEmptyLine(content)
		if preview == "" {
			return
		}
		widget.items = append(widget.items, registerItem{
			Key:     key,
			Preview: preview,
		})

		km[string(key)] = func(c wig.Context) {
			c.Editor.PopUi()
			c.Editor.ActiveRegister = key
			wig.CmdYankPut(c)
		}
	}

	// 1. Unnamed register '"'
	if un := wig.GetRegisterText(ctx, '"'); un != "" {
		addItem('"', un)
	}

	// 2. Dedicated yank register '0'
	if y0 := wig.GetRegisterText(ctx, '0'); y0 != "" {
		addItem('0', y0)
	}

	// 3. Small delete register '-'
	if sm := wig.GetRegisterText(ctx, '-'); sm != "" {
		addItem('-', sm)
	}

	// 4. Numbered delete history registers '1' - '9'
	for i := 1; i <= 9; i++ {
		r := rune('0' + i)
		if val := wig.GetRegisterText(ctx, r); val != "" {
			addItem(r, val)
		}
	}

	// 5. Named registers 'a' - 'z'
	var namedKeys []rune
	for k := range wig.NamedRegisters {
		if k >= 'a' && k <= 'z' {
			namedKeys = append(namedKeys, k)
		}
	}
	sort.Slice(namedKeys, func(i, j int) bool { return namedKeys[i] < namedKeys[j] })
	for _, k := range namedKeys {
		addItem(k, wig.NamedRegisters[k].Val)
	}

	// 6. Current file '%'
	if fn := wig.GetRegisterText(ctx, '%'); fn != "" {
		addItem('%', fn)
	}

	// 7. Alternate file '#'
	if alt := wig.GetRegisterText(ctx, '#'); alt != "" {
		addItem('#', alt)
	}

	// 8. Last Ex command ':'
	if cmd := wig.GetRegisterText(ctx, ':'); cmd != "" {
		addItem(':', cmd)
	}

	// 9. Last search pattern '/'
	if sch := wig.GetRegisterText(ctx, '/'); sch != "" {
		addItem('/', sch)
	}

	// 10. Last inserted text '.'
	if ins := wig.GetRegisterText(ctx, '.'); ins != "" {
		addItem('.', ins)
	}

	// 11. System clipboard '+' and '*'
	clipText, err := clipboard.ReadAll()
	if err == nil && strings.TrimSpace(clipText) != "" {
		addItem('+', clipText)
		addItem('*', clipText)
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
}

func (u *RegistersPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	lines := []string{}
	if len(u.items) == 0 {
		lines = append(lines, " No registers populated. ")
	} else {
		for _, item := range u.items {
			s := fmt.Sprintf(" '%c' : %s ", item.Key, item.Preview)
			lines = append(lines, s)
		}
	}

	boxH := len(lines) + 2
	boxW := int(float32(vw) * 0.9)
	if boxW > vw {
		boxW = vw
	}

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := vh - boxH - 2
	if y < 0 {
		y = 0
	}

	// Use default colors to match the marks popup exactly
	style := wig.Color("default")
	drawBox(view, x, y, boxW, boxH, style)

	// Title on top border
	view.SetContent(x+2, y, " Registers ", wig.Color("ui.popup.title"))

	textStyle := wig.Color("default")
	keyStyle := wig.Color("ui.mark")

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

			keyChar := string(item.Key)
			view.SetContent(cx, row, keyChar, keyStyle)
			cx += 1

			rest := fmt.Sprintf("' : %s ", item.Preview)
			view.SetContent(cx, row, rest, textStyle)
		}
	}
}
