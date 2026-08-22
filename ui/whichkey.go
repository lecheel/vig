package ui

import (
	"fmt"
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

func (w *WhichKey) getActionInfo(action any) (desc string, isGroup bool) {
	if km, ok := action.(wig.KeyMap); ok {
		return fmt.Sprintf("+prefix (%d)", len(km)), true
	}

	format := ""
	if w.e != nil {
		format = w.e.Config.WhichKeyFormat
	}

	val := reflect.ValueOf(action)
	if val.Kind() == reflect.Func {
		ptr := val.Pointer()
		for name, cmd := range wig.AllCommands {
			if cmd.Fn != nil && reflect.ValueOf(cmd.Fn).Pointer() == ptr {
				if format != "cmd" && format != "camel" && format != "camelcase" && cmd.Desc != "" {
					return cmd.Desc, false
				}
				return formatCmdName(name, format), false
			}
		}

		fn := runtime.FuncForPC(ptr)
		if fn != nil {
			name := fn.Name()
			parts := strings.Split(name, ".")
			if len(parts) > 0 {
				return formatCmdName(parts[len(parts)-1], format), false
			}
			return formatCmdName(name, format), false
		}
	}
	return "", false
}

func formatCmdName(name string, format string) string {
	switch strings.ToLower(format) {
	case "cmd":
		return name
	case "camel", "camelcase":
		return strings.TrimPrefix(name, "Cmd")
	default:
		name = strings.TrimPrefix(name, "Cmd")
		var result []rune
		runes := []rune(name)
		for i := 0; i < len(runes); i++ {
			r := runes[i]
			if i > 0 && r >= 'A' && r <= 'Z' {
				prev := runes[i-1]
				isPrevLower := prev >= 'a' && prev <= 'z'
				isPrevUpper := prev >= 'A' && prev <= 'Z'
				isNextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
				if isPrevLower || (isPrevUpper && isNextLower) {
					result = append(result, ' ')
				}
			}
			result = append(result, r)
		}
		return string(result)
	}
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

	if len(keys) == 0 {
		return
	}

	vw, vh := view.Size()

	// Multi-column calculations
	maxRows := max(vh/3, 8)
	if maxRows > vh-4 {
		maxRows = max(vh-4, 1)
	}

	numCols := (len(keys) + maxRows - 1) / maxRows
	if numCols < 1 {
		numCols = 1
	}

	numRows := (len(keys) + numCols - 1) / numCols
	if numRows < 1 {
		numRows = 1
	}

	type itemInfo struct {
		key     string
		desc    string
		isGroup bool
	}

	colItems := make([][]itemInfo, numCols)
	colWidths := make([]int, numCols)

	for i, k := range keys {
		c := i / numRows
		if c >= numCols {
			c = numCols - 1
		}
		desc, isGroup := w.getActionInfo(w.items[k])
		info := itemInfo{key: k, desc: desc, isGroup: isGroup}
		colItems[c] = append(colItems[c], info)

		itemWidth := len(k) + len(desc) + 4
		if itemWidth > colWidths[c] {
			colWidths[c] = itemWidth
		}
	}

	totalInnerW := 0
	for _, wCol := range colWidths {
		totalInnerW += wCol + 2
	}

	boxW := totalInnerW + 4
	if boxW < 36 {
		boxW = 36
	}
	if boxW > vw {
		boxW = vw
	}

	boxH := numRows + 2
	x := vw - boxW
	if x < 0 {
		x = 0
	}
	y := vh - boxH - 1
	if y < 0 {
		y = 0
	}

	bgStyle := wig.Color("default")
	keyStyle := wig.Color("ui.whichkey.key")
	groupStyle := wig.Color("ui.whichkey.group")
	if groupStyle == bgStyle {
		groupStyle = keyStyle
	}

	drawBox(view, x, y, boxW, boxH, bgStyle)

	// Title header
	title := fmt.Sprintf(" Which Key (%s) ", w.mode.String())
	if len(title) < boxW-4 {
		view.SetContent(x+2, y, title, keyStyle)
	}

	xOffset := x + 2
	for c, items := range colItems {
		for r, item := range items {
			yCur := y + r + 1
			xCur := xOffset

			view.SetContent(xCur, yCur, item.key, keyStyle)
			xCur += len(item.key)

			view.SetContent(xCur, yCur, " -> ", bgStyle)
			xCur += 4

			valStyle := bgStyle
			if item.isGroup {
				valStyle = groupStyle
			}
			view.SetContent(xCur, yCur, item.desc, valStyle)
		}
		xOffset += colWidths[c] + 2
	}
}
