// whichkey.go
package wig

import (
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

type WhichKey struct {
	e         *Editor
	keymap    *KeyHandler
	mode      Mode
	items     KeyMap
	startTime time.Time
}

func WhichKeyInit(e *Editor, keymap *KeyHandler, mode Mode, items KeyMap) *WhichKey {
	w := &WhichKey{
		e:         e,
		keymap:    keymap,
		mode:      mode,
		items:     items,
		startTime: time.Now(),
	}
	e.PushUi(w)

	// Schedule a redraw after 300ms. If the user types faster than that,
	// the popup will be closed or updated before this redraw matters.
	// If the popup is closed by then, Redraw simply renders the editor normally.
	go func() {
		time.Sleep(300 * time.Millisecond)
		w.e.Redraw()
	}()

	e.Redraw()
	return w
}

func (w *WhichKey) Close() {
	w.e.PopUiComponent(w)
	w.e.Redraw()
}

func (w *WhichKey) Plane() RenderPlane  { return PlaneEditor }
func (w *WhichKey) Mode() Mode          { return w.mode }
func (w *WhichKey) Keymap() *KeyHandler { return w.keymap }

func (w *WhichKey) Update(items KeyMap) {
	w.items = items
	w.startTime = time.Now() // Reset timer on each key press

	go func() {
		time.Sleep(300 * time.Millisecond)
		w.e.Redraw()
	}()
}

func getActionName(action any) string {
	// If it's another KeyMap, it means it opens another menu
	if _, ok := action.(KeyMap); ok {
		return "..."
	}

	val := reflect.ValueOf(action)
	if val.Kind() == reflect.Func {
		fn := runtime.FuncForPC(val.Pointer())
		if fn != nil {
			name := fn.Name()
			// Extract the function name, e.g. "CmdCursorLeft" from "github.com/firstrow/wig.CmdCursorLeft"
			parts := strings.Split(name, ".")
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
			return name
		}
	}
	return ""
}

func (w *WhichKey) Render(view View) {
	// Don't render until 300ms have passed since the last key press
	if time.Since(w.startTime) < 300*time.Millisecond {
		return
	}

	keys := make([]string, 0, len(w.items))
	for k := range w.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	maxLen := 0
	for _, k := range keys {
		desc := getActionName(w.items[k])
		lineLen := len(k) + len(desc) + 3 // +3 for " : "
		if lineLen > maxLen {
			maxLen = lineLen
		}
	}

	boxW := maxLen + 4
	if boxW < 30 {
		boxW = 30
	}

	boxH := len(keys) + 2
	if boxH < 3 {
		boxH = 3
	}

	vw, vh := view.Size()
	x := vw - boxW
	y := vh - boxH - 1 // -1 moves it one line up from the bottom

	bgStyle := Color("default")
	keyStyle := bgStyle.Foreground(tcell.ColorGreen)

	// Draw the box (border + filled interior) with the shared helper.
	// DrawBox takes inclusive end coordinates, so the box occupies
	// columns x..x+boxW-1 and rows y..y+boxH-1. The interior is already
	// filled with bgStyle, so we only need to overlay the key/desc text;
	// the trailing-space padding and right border are handled by DrawBox.
	DrawBox(view, x, y, x+boxW-1, y+boxH-1, bgStyle)

	for i, k := range keys {
		desc := getActionName(w.items[k])
		yCur := y + i + 1
		xCur := x + 2 // border at x, padding at x+1, content from x+2

		view.SetContent(xCur, yCur, k, keyStyle)
		xCur += len(k)

		view.SetContent(xCur, yCur, " : ", bgStyle)
		xCur += 3

		view.SetContent(xCur, yCur, desc, bgStyle)
	}
}
