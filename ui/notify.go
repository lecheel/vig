package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

type NotifyLevel int

const (
	NotifyInfo NotifyLevel = iota
	NotifySuccess
	NotifyWarn
	NotifyError
)

type NotifyItem struct {
	ID        int64
	Text      string
	Level     NotifyLevel
	CreatedAt time.Time
	Duration  time.Duration
}

type NotifyManager struct {
	mu       sync.Mutex
	items    []NotifyItem
	nextID   int64
	e        *wig.Editor
	isPushed bool
}

var GlobalNotify *NotifyManager

func InitNotifyManager(e *wig.Editor) *NotifyManager {
	nm := &NotifyManager{
		e:     e,
		items: make([]NotifyItem, 0),
	}
	GlobalNotify = nm
	return nm
}

func Notify(text string, level ...NotifyLevel) {
	if wig.EditorInst == nil {
		return
	}
	if GlobalNotify == nil {
		GlobalNotify = InitNotifyManager(wig.EditorInst)
	}
	lvl := NotifyInfo
	if len(level) > 0 {
		lvl = level[0]
	}
	GlobalNotify.Add(text, lvl, 5*time.Second)
}

func (nm *NotifyManager) Add(text string, level NotifyLevel, duration time.Duration) {
	nm.mu.Lock()
	nm.nextID++
	id := nm.nextID
	item := NotifyItem{
		ID:        id,
		Text:      text,
		Level:     level,
		CreatedAt: time.Now(),
		Duration:  duration,
	}
	nm.items = append(nm.items, item)

	if !nm.isPushed {
		nm.isPushed = true
		nm.e.PushUi(nm)
	}
	nm.mu.Unlock()

	nm.e.LogMessage(fmt.Sprintf("[Notify] Added id=%d: %s", id, text))
	nm.e.Redraw()

	time.AfterFunc(duration, func() {
		nm.mu.Lock()
		newItems := make([]NotifyItem, 0, len(nm.items))
		for _, it := range nm.items {
			if it.ID != id && time.Since(it.CreatedAt) < it.Duration {
				newItems = append(newItems, it)
			}
		}
		nm.items = newItems
		if len(nm.items) == 0 && nm.isPushed {
			nm.isPushed = false
			nm.e.PopUiComponent(nm)
		}
		nm.mu.Unlock()
		nm.e.Redraw()
	})
}

func (nm *NotifyManager) Plane() wig.RenderPlane { return wig.PlaneEditor }

func (nm *NotifyManager) Mode() wig.Mode {
	if nm.e != nil && nm.e.ActiveBuffer() != nil {
		return nm.e.ActiveBuffer().Mode()
	}
	return wig.MODE_NORMAL
}

func (nm *NotifyManager) Keymap() *wig.KeyHandler {
	if nm.e != nil {
		if nm.e.ActiveWindow() != nil && nm.e.ActiveWindow().Buffer() != nil && nm.e.ActiveWindow().Buffer().KeyHandler != nil {
			return nm.e.ActiveWindow().Buffer().KeyHandler
		}
		return nm.e.Keys
	}
	return nil
}

func (nm *NotifyManager) Render(view wig.View) {
	nm.mu.Lock()
	now := time.Now()
	active := make([]NotifyItem, 0, len(nm.items))
	for _, it := range nm.items {
		if now.Sub(it.CreatedAt) < it.Duration {
			active = append(active, it)
		}
	}
	nm.items = active
	items := make([]NotifyItem, len(active))
	copy(items, active)
	nm.mu.Unlock()

	if len(items) == 0 {
		return
	}

	vw, vh := view.Size()
	startY := 1

	for i, it := range items {
		yCur := startY + i*3
		if yCur+2 >= vh-1 {
			break
		}

		var icon string
		var iconColor tcell.Color

		switch it.Level {
		case NotifySuccess:
			icon = "[✓]"
			iconColor = tcell.ColorGreen
		case NotifyWarn:
			icon = "[⚠]"
			iconColor = tcell.ColorYellow
		case NotifyError:
			icon = "[✕]"
			iconColor = tcell.ColorRed
		default:
			icon = "[ℹ]"
			iconColor = tcell.ColorAqua
		}

		badgeStyle := tcell.StyleDefault.Foreground(iconColor).Bold(true)
		textStyle := wig.Color("default")
		boxBg := wig.Color("ui.popup")

		iconLen := utf8.RuneCountInString(icon)
		textLen := utf8.RuneCountInString(it.Text)
		// 2 border chars + 2 left padding + 1 gap + 2 right padding
		msgWidth := iconLen + 1 + textLen + 6
		if msgWidth < 24 {
			msgWidth = 24
		}
		if msgWidth > vw-4 {
			msgWidth = vw - 4
		}

		x := vw - msgWidth - 2
		if x < 0 {
			x = 0
		}

		// Draw border
		drawBox(view, x, yCur, msgWidth, 3, boxBg)

		// Clear the entire inside row with the popup background style
		innerWidth := msgWidth - 2
		if innerWidth > 0 {
			view.SetContent(x+1, yCur+1, strings.Repeat(" ", innerWidth), boxBg)
		}

		// Draw icon and text safely inside the box with comfortable margins
		view.SetContent(x+2, yCur+1, icon, badgeStyle)
		xText := x + 2 + iconLen + 1
		view.SetContent(xText, yCur+1, it.Text, textStyle)
	}
}
