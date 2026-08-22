package ui

import (
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/firstrow/wig"
)

type WhichKey struct {
	e         *wig.Editor
	keymap    *wig.KeyHandler
	mode      wig.Mode
	items     wig.KeyMap
	startTime time.Time
}

func WhichKeyInit(e *wig.Editor, keymap *wig.KeyHandler, mode wig.Mode, items wig.KeyMap) *WhichKey {
	w := &WhichKey{
		e:         e,
		keymap:    keymap,
		mode:      mode,
		items:     items,
		startTime: time.Now(),
	}
	e.PushUi(w)

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

func (w *WhichKey) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (w *WhichKey) Mode() wig.Mode          { return w.mode }
func (w *WhichKey) Keymap() *wig.KeyHandler { return w.keymap }

func (w *WhichKey) Update(items wig.KeyMap) {
	w.items = items
	w.startTime = time.Now()

	go func() {
		time.Sleep(300 * time.Millisecond)
		w.e.Redraw()
	}()
}

func getActionName(action any) string {
	if _, ok := action.(wig.KeyMap); ok {
		return "..."
	}

	val := reflect.ValueOf(action)
	if val.Kind() == reflect.Func {
		fn := runtime.FuncForPC(val.Pointer())
		if fn != nil {
			name := fn.Name()
			parts := strings.Split(name, ".")
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
			return name
		}
	}
	return ""
}

func (w *WhichKey) Render(view wig.View) {
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
		lineLen := len(k) + len(desc) + 3
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
	y := vh - boxH - 1

	bgStyle := wig.Color("default")
	keyStyle := wig.Color("ui.whichkey.key")

	drawBox(view, x, y, x+boxW-1, y+boxH-1, bgStyle)

	for i, k := range keys {
		desc := getActionName(w.items[k])
		yCur := y + i + 1
		xCur := x + 2

		view.SetContent(xCur, yCur, k, keyStyle)
		xCur += len(k)

		view.SetContent(xCur, yCur, " : ", bgStyle)
		xCur += 3

		view.SetContent(xCur, yCur, desc, bgStyle)
	}
}
