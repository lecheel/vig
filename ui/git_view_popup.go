package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
)

// GitViewCallbacks groups the action callbacks the git status popup invokes
// when the user presses a key. The popup itself is deliberately unaware of
// git: all git commands and refresh logic live in the `commands` package,
// which passes callbacks in here.
type GitViewCallbacks struct {
	// OnEnter is fired on Enter. The item may be a file, branch or stash —
	// the callback dispatches based on item.Type.
	OnEnter func(ctx wig.Context, item *wig.GitViewItem)
	// OnStage is fired on `s`: stage/unstage a file.
	OnStage func(ctx wig.Context, item *wig.GitViewItem)
	// OnDiff is fired on `d`: preview a file or stash diff.
	OnDiff func(ctx wig.Context, item *wig.GitViewItem)
	// OnRefresh is fired on `r` and after mutating actions. It must return
	// the new item list so the popup can re-render.
	OnRefresh func(ctx wig.Context) []wig.GitViewItem
	// OnPush is fired on `p`.
	OnPush func(ctx wig.Context)
	// OnCommit is fired on `c` (useAI=false) or `a` (useAI=true).
	OnCommit func(ctx wig.Context, useAI bool)
	// OnStash is fired on `z`: stash unstaged changes.
	OnStash func(ctx wig.Context)
}

// GitViewPopupWidget renders the git status panel as a centered popup
// occupying ~90% of the screen width. It replaces the older buffer-based
// git view: same keys, same layout, same actions — just as a floating
// overlay instead of a full buffer with its own window.
type GitViewPopupWidget struct {
	e            *wig.Editor
	keymap       *wig.KeyHandler
	items        []wig.GitViewItem
	activeIdx    int
	scrollOffset int
	cb           GitViewCallbacks
}

func (u *GitViewPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *GitViewPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *GitViewPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

// gitItemSelectable reports whether the cursor may land on an item. Headers,
// separators and empty placeholders are skipped by every navigation key.
func gitItemSelectable(t string) bool {
	return t == "file" || t == "branch" || t == "stash"
}

func gitFirstSelectable(items []wig.GitViewItem, from int) int {
	for i := from; i < len(items); i++ {
		if gitItemSelectable(items[i].Type) {
			return i
		}
	}
	return 0
}

func GitViewPopupInit(ctx wig.Context, items []wig.GitViewItem, cb GitViewCallbacks) *GitViewPopupWidget {
	widget := &GitViewPopupWidget{
		e:     ctx.Editor,
		items: items,
		cb:    cb,
	}
	widget.activeIdx = gitFirstSelectable(items, 0)

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) { ctx.Editor.PopUiComponent(widget) },
		"q":   func(ctx wig.Context) { ctx.Editor.PopUiComponent(widget) },

		"j":      func(ctx wig.Context) { widget.moveDown(1) },
		"Down":   func(ctx wig.Context) { widget.moveDown(1) },
		"k":      func(ctx wig.Context) { widget.moveUp(1) },
		"Up":     func(ctx wig.Context) { widget.moveUp(1) },
		"ctrl+d": func(ctx wig.Context) { widget.moveDown(8) },
		"PgDn":   func(ctx wig.Context) { widget.moveDown(8) },
		"ctrl+u": func(ctx wig.Context) { widget.moveUp(8) },
		"PgUp":   func(ctx wig.Context) { widget.moveUp(8) },
		"g":      func(ctx wig.Context) { widget.goTop() },
		"Home":   func(ctx wig.Context) { widget.goTop() },
		"G":      func(ctx wig.Context) { widget.goBottom() },
		"End":    func(ctx wig.Context) { widget.goBottom() },

		"l": func(ctx wig.Context) { widget.jumpSection(1) },
		"L": func(ctx wig.Context) { widget.jumpSection(-1) },

		"Enter": func(ctx wig.Context) {
			if item := widget.activeItem(); item != nil && widget.cb.OnEnter != nil {
				widget.cb.OnEnter(ctx, item)
			}
		},
		"s": func(ctx wig.Context) {
			if item := widget.activeItem(); item != nil && widget.cb.OnStage != nil {
				widget.cb.OnStage(ctx, item)
				widget.refresh(ctx)
			}
		},
		"d": func(ctx wig.Context) {
			if item := widget.activeItem(); item != nil && widget.cb.OnDiff != nil {
				widget.cb.OnDiff(ctx, item)
			}
		},
		"p": func(ctx wig.Context) {
			if widget.cb.OnPush != nil {
				widget.cb.OnPush(ctx)
			}
		},
		"c": func(ctx wig.Context) {
			if widget.cb.OnCommit != nil {
				ctx.Editor.PopUiComponent(widget)
				widget.cb.OnCommit(ctx, false)
			}
		},
		"a": func(ctx wig.Context) {
			if widget.cb.OnCommit != nil {
				ctx.Editor.PopUiComponent(widget)
				widget.cb.OnCommit(ctx, true)
			}
		},
		"z": func(ctx wig.Context) {
			if widget.cb.OnStash != nil {
				widget.cb.OnStash(ctx)
				widget.refresh(ctx)
			}
		},
		"r": func(ctx wig.Context) { widget.refresh(ctx) },
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
	ctx.Editor.Redraw()
	return widget
}

// Refresh re-queries the callback for a fresh item list. Called by the
// widget itself on `r`, and by commands that mutate git state from a
// Confirm prompt so the popup updates when the prompt resolves.
func (u *GitViewPopupWidget) Refresh(ctx wig.Context) {
	u.refresh(ctx)
}

func (u *GitViewPopupWidget) refresh(ctx wig.Context) {
	if u.cb.OnRefresh == nil {
		return
	}
	u.items = u.cb.OnRefresh(ctx)
	if len(u.items) == 0 {
		u.activeIdx = 0
		u.scrollOffset = 0
		u.e.Redraw()
		return
	}
	if u.activeIdx >= len(u.items) {
		u.activeIdx = len(u.items) - 1
	}
	if u.activeIdx < 0 {
		u.activeIdx = 0
	}
	if !gitItemSelectable(u.items[u.activeIdx].Type) {
		u.activeIdx = gitFirstSelectable(u.items, u.activeIdx)
	}
	u.e.Redraw()
}

func (u *GitViewPopupWidget) activeItem() *wig.GitViewItem {
	if u.activeIdx < 0 || u.activeIdx >= len(u.items) {
		return nil
	}
	it := &u.items[u.activeIdx]
	if !gitItemSelectable(it.Type) {
		return nil
	}
	return it
}

func (u *GitViewPopupWidget) moveDown(n int) {
	if len(u.items) == 0 {
		return
	}
	for step := 0; step < n; step++ {
		moved := false
		for j := u.activeIdx + 1; j < len(u.items); j++ {
			if gitItemSelectable(u.items[j].Type) {
				u.activeIdx = j
				moved = true
				break
			}
		}
		if !moved {
			break
		}
	}
	u.e.Redraw()
}

func (u *GitViewPopupWidget) moveUp(n int) {
	if len(u.items) == 0 {
		return
	}
	for step := 0; step < n; step++ {
		moved := false
		for j := u.activeIdx - 1; j >= 0; j-- {
			if gitItemSelectable(u.items[j].Type) {
				u.activeIdx = j
				moved = true
				break
			}
		}
		if !moved {
			break
		}
	}
	u.e.Redraw()
}

func (u *GitViewPopupWidget) goTop() {
	u.activeIdx = gitFirstSelectable(u.items, 0)
	u.scrollOffset = 0
	u.e.Redraw()
}

func (u *GitViewPopupWidget) goBottom() {
	for j := len(u.items) - 1; j >= 0; j-- {
		if gitItemSelectable(u.items[j].Type) {
			u.activeIdx = j
			break
		}
	}
	u.e.Redraw()
}

// jumpSection moves the cursor to the first selectable item of the next
// (dir > 0) or previous (dir < 0) section header.
func (u *GitViewPopupWidget) jumpSection(dir int) {
	if len(u.items) == 0 {
		return
	}
	if dir > 0 {
		for i := u.activeIdx + 1; i < len(u.items); i++ {
			if u.items[i].Type == "header" {
				u.activeIdx = gitFirstSelectable(u.items, i+1)
				u.e.Redraw()
				return
			}
		}
	} else {
		for i := u.activeIdx - 1; i >= 0; i-- {
			if u.items[i].Type == "header" {
				u.activeIdx = gitFirstSelectable(u.items, i+1)
				u.e.Redraw()
				return
			}
		}
	}
}

// gitViewHint is the bottom-of-popup key hint. Kept short so it fits even
// on narrow terminals.
const gitViewHint = " [Enter] Open  [s] Stage  [d] Diff  [c] Commit  [a] AI  [p] Push  [z] Stash  [r] Refresh  [Esc] Close"

func (u *GitViewPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	// 90% screen width, centered. Height is bounded so the popup stays a
	// popup — not a full-height overlay — even on tall terminals.
	boxW := int(float32(vw) * 0.90)
	if boxW < 40 {
		boxW = 40
	}
	if boxW > vw {
		boxW = vw
	}
	boxH := min(26, vh-4)
	if boxH < 8 {
		boxH = 8
	}
	if boxH > vh {
		boxH = vh
	}
	x := (vw - boxW) / 2
	y := (vh - boxH) / 2

	style := wig.Color("default")
	drawBox(view, x, y, boxW, boxH, style)

	title := fmt.Sprintf(" Git (%d) ", len(u.items))
	view.SetContent(x+2, y, truncate(title, boxW-4), wig.Color("ui.popup.title"))

	// Hint on the bottom border, replacing the box edge chars it overlaps.
	hintStyle := wig.Color("ui.linenr")
	hint := truncate(gitViewHint, boxW-4)
	view.SetContent(x+2, y+boxH-1, hint, hintStyle)

	// Content area: rows y+1 .. y+boxH-2 (excludes top and bottom border).
	visibleRows := boxH - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Keep the active item inside the visible window.
	if u.activeIdx < u.scrollOffset {
		u.scrollOffset = u.activeIdx
	}
	if u.activeIdx >= u.scrollOffset+visibleRows {
		u.scrollOffset = u.activeIdx - visibleRows + 1
	}
	if u.scrollOffset < 0 {
		u.scrollOffset = 0
	}

	endIdx := min(u.scrollOffset+visibleRows, len(u.items))
	innerW := boxW - 2
	for i := u.scrollOffset; i < endIdx; i++ {
		it := u.items[i]
		row := y + 1 + (i - u.scrollOffset)
		cx := x + 1

		isActive := i == u.activeIdx && gitItemSelectable(it.Type)
		cursor := "  "
		if i == u.activeIdx {
			cursor = "> "
		}

		itemStyle := wig.Color("default")
		var lineText string
		switch it.Type {
		case "header":
			lineText = cursor + "── " + it.Label + " "
			itemStyle = wig.Color("ui.linenr.selected")
		case "separator":
			lineText = ""
		case "empty":
			lineText = cursor + "   " + it.Label
			itemStyle = wig.Color("comment")
		case "file":
			lineText = fmt.Sprintf("%s%s  %s", cursor, it.Code, it.FilePath)
		case "branch":
			lineText = cursor + it.Label
		case "stash":
			lineText = fmt.Sprintf("%s%s: %s", cursor, it.StashRef, it.Label)
		default:
			lineText = cursor + it.Label
		}

		if isActive {
			itemStyle = wig.Color("ui.menu.selected")
		}
		if lineText == "" {
			// Clear the row so stale cells from a previous frame don't
			// bleed through. drawBox only fills y+1..y+boxH-2 on init.
			view.SetContent(cx, row, strings.Repeat(" ", innerW), wig.Color("default"))
			continue
		}
		view.SetContent(cx, row, truncate(lineText, innerW), itemStyle)
	}
}
