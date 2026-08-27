package wig

import (
	"github.com/gdamore/tcell/v2"
	"strings"
	"unicode"
)

func HandleInsertKey(ctx Context, ev *tcell.EventKey) {
	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	ch := ev.Rune()

	if ev.Key() == tcell.KeyCtrlJ {
		ev = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	}

	{
		if ctx.Buf.Mode() != MODE_INSERT {
			return
		}
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return
		}

		if ev.Modifiers()&tcell.ModAlt != 0 {
			return
		}
		if ev.Modifiers()&tcell.ModMeta != 0 {
			return
		}
	}

	if ev.Key() == tcell.KeyEnter {
		ch = '\n'
	}

	{
		if ch == '\t' {
			if Tabstopped(ctx) {
				TabstopNext(ctx)
				return
			}

			// Trigger path completion if word starts with ./ or ../
			lineStr := line.Value.String()
			wordStart := cur.Char
			for wordStart > 0 {
				r := line.Value[wordStart-1]
				if r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
					wordStart--
				} else {
					break
				}
			}
			currentWord := lineStr[wordStart:cur.Char]
			if strings.HasPrefix(currentWord, "./") || strings.HasPrefix(currentWord, "../") {
				if ctx.Editor.AutocompleteTrigger(ctx) {
					return
				}
			}

			if strings.TrimSpace(line.Value.String()) == "" {
				goto insertChar
			}
			if cur.Char >= len(line.Value.String())-1 {
				if ctx.Editor.AutocompleteTrigger(ctx) {
					return
				}
			}
			goto insertChar
		}
	}

insertChar:

	if ch == 0 {
		return
	}

	if ch == '\t' {
		indentInsert(ctx)
		return
	}

	if ev.Key() == tcell.KeyDelete {
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		line := CursorLine(ctx.Buf, cur)
		if line == nil {
			return
		}
		if cur.Char < len(line.Value)-1 {
			TextDelete(ctx.Buf, &Selection{
				Start: Cursor{Line: cur.Line, Char: cur.Char},
				End:   Cursor{Line: cur.Line, Char: cur.Char + 1},
			})
		} else if line.Next() != nil {
			TextDelete(ctx.Buf, &Selection{
				Start: Cursor{Line: cur.Line, Char: cur.Char},
				End:   Cursor{Line: cur.Line + 1, Char: 0},
			})
		}
		return
	}

	if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
		start := *cur
		start.Char--

		if start.Char < 0 {
			if line.Prev() == nil {
				return
			}

			cur.Line--
			CmdGotoLineEnd(ctx)

			// delete \n on prev line
			TextDelete(ctx.Buf, &Selection{
				Start: Cursor{Line: start.Line - 1, Char: len(line.Prev().Value) - 1},
				End:   Cursor{Line: start.Line - 1, Char: len(line.Prev().Value)},
			})

			return
		}

		TextDelete(ctx.Buf, &Selection{
			Start: start,
			End:   *cur,
		})
		if cur.Char > 0 {
			cur.Char--
		}
		return
	}

	SelectionDelete(ctx)
	TextInsert(ctx.Buf, line, cur.Char, string(ch))

	if ev.Key() == tcell.KeyEnter {
		CmdCursorLineDown(ctx)
		CmdCursorBeginningOfTheLine(ctx)
		indent(ctx)
		return
	}

	if cur.Char < len(line.Value) {
		cur.Char++
		cur.PreserveCharPosition = cur.Char
	}
}
