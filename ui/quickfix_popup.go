package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/firstrow/wig"
)

type QuickfixItem struct {
	FilePath string
	Line     int // 0-indexed
	Char     int // 0-indexed
	Message  string
}

type QuickfixPopupWidget struct {
	e            *wig.Editor
	keymap       *wig.KeyHandler
	items        []QuickfixItem
	activeIdx    int
	scrollOffset int
	onSelect     func(item QuickfixItem)
}

func (u *QuickfixPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *QuickfixPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *QuickfixPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

func QuickfixPopupInit(ctx wig.Context, items []QuickfixItem, onSelect func(item QuickfixItem)) *QuickfixPopupWidget {
	widget := &QuickfixPopupWidget{
		e:        ctx.Editor,
		items:    items,
		onSelect: onSelect,
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
		"Tab": func(ctx wig.Context) {
			widget.moveDown(1)
		},
		"k": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Up": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Backtab": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"PgDn": func(ctx wig.Context) {
			widget.moveDown(8)
		},
		"PgUp": func(ctx wig.Context) {
			widget.moveUp(8)
		},
		"Home": func(ctx wig.Context) {
			widget.activeIdx = 0
			widget.scrollOffset = 0
			widget.e.Redraw()
		},
		"End": func(ctx wig.Context) {
			if len(widget.items) > 0 {
				widget.activeIdx = len(widget.items) - 1
				widget.ensureVisible()
				widget.e.Redraw()
			}
		},
		"Enter": func(ctx wig.Context) {
			if widget.activeIdx >= 0 && widget.activeIdx < len(widget.items) {
				ctx.Editor.PopUi()
				if widget.onSelect != nil {
					widget.onSelect(widget.items[widget.activeIdx])
				}
			}
		},
	}

	// 1-8 direct jump shortcuts (like marks/bookmarks)
	for i := 1; i <= 8; i++ {
		slot := i
		km[fmt.Sprintf("%d", slot)] = func(ctx wig.Context) {
			targetIdx := widget.scrollOffset + slot - 1
			if targetIdx >= 0 && targetIdx < len(widget.items) {
				ctx.Editor.PopUi()
				if widget.onSelect != nil {
					widget.onSelect(widget.items[targetIdx])
				}
			}
		}
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
	return widget
}

func (u *QuickfixPopupWidget) moveDown(n int) {
	if len(u.items) == 0 {
		return
	}
	u.activeIdx = min(u.activeIdx+n, len(u.items)-1)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *QuickfixPopupWidget) moveUp(n int) {
	if len(u.items) == 0 {
		return
	}
	u.activeIdx = max(u.activeIdx-n, 0)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *QuickfixPopupWidget) ensureVisible() {
	maxVis := 8
	if u.activeIdx < u.scrollOffset {
		u.scrollOffset = u.activeIdx
	} else if u.activeIdx >= u.scrollOffset+maxVis {
		u.scrollOffset = u.activeIdx - maxVis + 1
	}
}

func (u *QuickfixPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	const maxItems = 8
	visCount := len(u.items)
	if visCount > maxItems {
		visCount = maxItems
	}
	if visCount == 0 {
		visCount = 1
	}

	boxH := visCount + 2
	boxW := int(float32(vw) * 0.9)
	if boxW > vw {
		boxW = vw
	}

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	// Grow bottom-up from statusbar
	y := vh - boxH - 2
	if y < 0 {
		y = 0
	}

	style := wig.Color("default")
	drawBox(view, x, y, boxW, boxH, style)

	title := fmt.Sprintf(" Quickfix (%d) [1-8 / j / k / Enter / Esc] ", len(u.items))
	view.SetContent(x+2, y, title, wig.Color("ui.popup.title"))

	if len(u.items) == 0 {
		view.SetContent(x+2, y+1, " No diagnostics found. Esc to close. ", wig.Color("comment"))
		return
	}

	endIdx := min(u.scrollOffset+maxItems, len(u.items))
	for i, item := range u.items[u.scrollOffset:endIdx] {
		actualIdx := u.scrollOffset + i
		row := y + 1 + i
		cx := x + 1

		isActive := actualIdx == u.activeIdx

		cursorPrefix := "  "
		if isActive {
			cursorPrefix = "> "
		}

		numPrefix := fmt.Sprintf("%d ", i+1)
		locPrefix := fmt.Sprintf("%s:%d:%d ", filepath.Base(item.FilePath), item.Line+1, item.Char)

		cursorStyle := wig.Color("ui.linenr.selected")
		numStyle := wig.Color("ui.mark")
		locStyle := wig.Color("ui.linenr")
		textStyle := wig.Color("default")

		if isActive {
			textStyle = wig.Color("ui.menu.selected")
		}

		view.SetContent(cx, row, cursorPrefix, cursorStyle)
		cx += len(cursorPrefix)

		view.SetContent(cx, row, numPrefix, numStyle)
		cx += len(numPrefix)

		view.SetContent(cx, row, locPrefix, locStyle)
		cx += len(locPrefix)

		availableW := (x + boxW - 1) - cx
		if availableW > 0 {
			cleanMsg := strings.ReplaceAll(item.Message, "\n", " ")
			view.SetContent(cx, row, truncate(cleanMsg, availableW), textStyle)
		}
	}
}
