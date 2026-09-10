package ui

import (
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
)

// LineEditor is a minimal single-line text buffer with a cursor position.
// It implements the subset of readline/emacs bindings shared by the `:`
// command line and the `/` search prompt: character navigation, home/end,
// backspace/delete, Shift-Insert paste, and the kill shortcuts (Ctrl-U/K/W).
//
// Component-specific keys (Enter, Esc, Tab, history, registers, ...) are
// handled by the owning widget before falling through to HandleReadlineKey.
type LineEditor struct {
	chBuf     []rune
	cursorPos int
}

// NewLineEditor returns a LineEditor initialized with the given text and
// the cursor at the end.
func NewLineEditor(initial string) *LineEditor {
	le := &LineEditor{}
	if initial != "" {
		le.chBuf = []rune(initial)
		le.cursorPos = len(le.chBuf)
	}
	return le
}

func (le *LineEditor) Text() string   { return string(le.chBuf) }
func (le *LineEditor) Runes() []rune  { return le.chBuf }
func (le *LineEditor) CursorPos() int { return le.cursorPos }
func (le *LineEditor) Len() int       { return len(le.chBuf) }

// SetText replaces the buffer contents and places the cursor at the end.
func (le *LineEditor) SetText(s string) {
	le.chBuf = []rune(s)
	le.cursorPos = len(le.chBuf)
}

// Clear empties the buffer and resets the cursor.
func (le *LineEditor) Clear() {
	le.chBuf = le.chBuf[:0]
	le.cursorPos = 0
}

// InsertRune inserts r at the cursor and advances the cursor.
func (le *LineEditor) InsertRune(r rune) {
	le.chBuf = append(le.chBuf, 0)
	copy(le.chBuf[le.cursorPos+1:], le.chBuf[le.cursorPos:])
	le.chBuf[le.cursorPos] = r
	le.cursorPos++
}

// InsertText inserts s at the cursor and advances the cursor.
func (le *LineEditor) InsertText(s string) {
	if s == "" {
		return
	}
	runes := []rune(s)
	newBuf := make([]rune, len(le.chBuf)+len(runes))
	copy(newBuf, le.chBuf[:le.cursorPos])
	copy(newBuf[le.cursorPos:], runes)
	copy(newBuf[le.cursorPos+len(runes):], le.chBuf[le.cursorPos:])
	le.chBuf = newBuf
	le.cursorPos += len(runes)
}

// Backspace deletes the rune before the cursor. Reports whether the buffer changed.
func (le *LineEditor) Backspace() bool {
	if le.cursorPos == 0 {
		return false
	}
	le.chBuf = append(le.chBuf[:le.cursorPos-1], le.chBuf[le.cursorPos:]...)
	le.cursorPos--
	return true
}

// Delete removes the rune under the cursor. Reports whether the buffer changed.
func (le *LineEditor) Delete() bool {
	if le.cursorPos >= len(le.chBuf) {
		return false
	}
	le.chBuf = append(le.chBuf[:le.cursorPos], le.chBuf[le.cursorPos+1:]...)
	return true
}

func (le *LineEditor) Left() bool {
	if le.cursorPos == 0 {
		return false
	}
	le.cursorPos--
	return true
}

func (le *LineEditor) Right() bool {
	if le.cursorPos >= len(le.chBuf) {
		return false
	}
	le.cursorPos++
	return true
}

func (le *LineEditor) Home() { le.cursorPos = 0 }
func (le *LineEditor) End()  { le.cursorPos = len(le.chBuf) }

// KillToStart deletes from the cursor to the start of the buffer (Ctrl-U).
func (le *LineEditor) KillToStart() bool {
	if le.cursorPos == 0 {
		return false
	}
	le.chBuf = le.chBuf[le.cursorPos:]
	le.cursorPos = 0
	return true
}

// KillToEnd deletes from the cursor to the end of the buffer (Ctrl-K).
func (le *LineEditor) KillToEnd() bool {
	if le.cursorPos >= len(le.chBuf) {
		return false
	}
	le.chBuf = le.chBuf[:le.cursorPos]
	return true
}

// KillWord deletes the whitespace-delimited word before the cursor (Ctrl-W).
func (le *LineEditor) KillWord() bool {
	if le.cursorPos == 0 {
		return false
	}
	start := le.cursorPos
	for start > 0 && le.chBuf[start-1] == ' ' {
		start--
	}
	for start > 0 && le.chBuf[start-1] != ' ' {
		start--
	}
	le.chBuf = append(le.chBuf[:start], le.chBuf[le.cursorPos:]...)
	le.cursorPos = start
	return true
}

// HandleReadlineKey processes a key event using standard readline/emacs
// bindings. Returns (handled, changed): handled reports whether the key was
// recognized and consumed; changed reports whether the buffer contents
// changed so callers can refresh dependent state (redraw, live search, ...).
//
// Component-specific keys (Enter, Esc, Tab, history, registers, ...) must
// be handled by the caller before falling through to this method.
func (le *LineEditor) HandleReadlineKey(ev *tcell.EventKey) (handled, changed bool) {
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		switch ev.Key() {
		case tcell.KeyCtrlA:
			le.Home()
		case tcell.KeyCtrlE:
			le.End()
		case tcell.KeyCtrlB:
			le.Left()
		case tcell.KeyCtrlF:
			le.Right()
		case tcell.KeyCtrlD:
			changed = le.Delete()
		case tcell.KeyCtrlU:
			changed = le.KillToStart()
		case tcell.KeyCtrlK:
			changed = le.KillToEnd()
		case tcell.KeyCtrlW:
			changed = le.KillWord()
		default:
			return false, false
		}
		return true, changed
	}
	if ev.Modifiers()&tcell.ModAlt != 0 || ev.Modifiers()&tcell.ModMeta != 0 {
		return false, false
	}
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return true, le.Backspace()
	case tcell.KeyDelete:
		return true, le.Delete()
	case tcell.KeyLeft:
		le.Left()
		return true, false
	case tcell.KeyRight:
		le.Right()
		return true, false
	case tcell.KeyHome:
		le.Home()
		return true, false
	case tcell.KeyEnd:
		le.End()
		return true, false
	case tcell.KeyInsert:
		if ev.Modifiers()&tcell.ModShift != 0 {
			if text, err := clipboard.ReadAll(); err == nil && text != "" {
				le.InsertText(text)
				return true, true
			}
			return true, false
		}
		return false, false
	case tcell.KeyRune:
		le.InsertRune(ev.Rune())
		return true, true
	}
	return false, false
}
