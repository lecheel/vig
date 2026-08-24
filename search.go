package wig

import (
	"sort"

	str "github.com/boyter/go-string"
)

// CountMatches counts all occurrences of pattern in buf and returns
// the 1-based index of the match closest to or at the current cursor.
func CountMatches(buf *Buffer, cur *Cursor, pattern string) (current int, total int) {
	if pattern == "" || buf == nil {
		return 0, 0
	}

	curLine := 0
	curChar := 0
	if cur != nil {
		curLine = cur.Line
		curChar = cur.Char
	}

	line := buf.Lines.First()
	lineNum := 0

	for line != nil {
		matches := str.IndexAllIgnoreCase(string(line.Value), pattern, -1)
		for _, m := range matches {
			if len(m) == 0 {
				continue
			}
			total++
			startChar := m[0]
			if lineNum < curLine || (lineNum == curLine && startChar <= curChar) {
				current = total
			}
		}
		line = line.Next()
		lineNum++
	}

	if current == 0 && total > 0 {
		current = 1
	}

	return current, total
}

// SearchFrom searches for pattern starting at startPos (inclusive).
// If not found towards the end of file, wraps around from line 0.
func SearchFrom(ctx Context, startPos Cursor, pattern string) bool {
	if pattern == "" || ctx.Buf == nil {
		return false
	}
	defer CmdEnsureCursorVisible(ctx)

	cur := ContextCursorGet(ctx)
	lineNum := startPos.Line
	line := CursorLineByNum(ctx.Buf, lineNum)
	if line == nil {
		return false
	}

	from := max(0, startPos.Char)
	haystack := string(line.Value.Range(from, EOL))

	// Search from startPos to EOF
	for line != nil {
		matches := str.IndexAllIgnoreCase(haystack, pattern, 1)
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				return matches[i][0] < matches[j][0]
			})
			cur.Line = lineNum
			cur.Char = matches[0][0] + from
			cur.PreserveCharPosition = cur.Char
			SelectionExtend(ctx.Buf, cur)
			return true
		}

		line = line.Next()
		if line == nil {
			break
		}
		lineNum++
		from = 0
		haystack = string(line.Value)
	}

	// Wrap around from line 0 up to startPos
	line = ctx.Buf.Lines.First()
	lineNum = 0
	for line != nil && lineNum <= startPos.Line {
		haystack = string(line.Value)
		matches := str.IndexAllIgnoreCase(haystack, pattern, 1)
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				return matches[i][0] < matches[j][0]
			})
			cur.Line = lineNum
			cur.Char = matches[0][0]
			cur.PreserveCharPosition = cur.Char
			SelectionExtend(ctx.Buf, cur)
			return true
		}
		line = line.Next()
		lineNum++
	}

	return false
}

// Move cursor to the next search pattern match
func SearchNext(ctx Context, pattern string) {
	defer CmdEnsureCursorVisible(ctx)

	cur := ContextCursorGet(ctx)
	if cur == nil || ctx.Buf == nil {
		return
	}
	origCur := *cur
	line := CursorLine(ctx.Buf, cur)
	lineNum := cur.Line
	from := cur.Char + 1
	haystack := string(line.Value.Range(from, EOL))

	for line != nil {
		matches := str.IndexAllIgnoreCase(haystack, pattern, 1)
		if len(matches) == 0 {
			line = line.Next()
			if line == nil {
				break
			}
			lineNum++
			from = 0
			haystack = string(line.Value)
			continue
		}

		sort.Slice(matches, func(i, j int) bool {
			return matches[i][0] < matches[j][0]
		})

		if ctx.Editor != nil && ctx.Editor.ActiveWindow() != nil {
			ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, &origCur)
		}
		cur.Line = lineNum
		cur.Char = matches[0][0] + from
		cur.PreserveCharPosition = cur.Char
		if ctx.Editor != nil && ctx.Editor.ActiveWindow() != nil {
			ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, cur)
		}
		SelectionExtend(ctx.Buf, cur)
		break
	}
}

func SearchPrev(ctx Context, pattern string) {
	defer CmdEnsureCursorVisible(ctx)

	cur := ContextCursorGet(ctx)
	if cur == nil || ctx.Buf == nil {
		return
	}
	origCur := *cur
	line := CursorLine(ctx.Buf, cur)

	ln := cur.Line
	haystack := string(line.Value.Range(0, cur.Char-1))

	for line != nil {
		matches := str.IndexAllIgnoreCase(haystack, pattern, -1)
		if len(matches) == 0 {
			line = line.Prev()
			if line == nil {
				break
			}
			ln--
			haystack = string(line.Value)
			continue
		}

		sort.Slice(matches, func(i, j int) bool {
			return matches[i][0] > matches[j][0]
		})

		if ctx.Editor != nil && ctx.Editor.ActiveWindow() != nil {
			ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, &origCur)
		}
		cur.Line = ln
		cur.Char = matches[0][0]
		cur.PreserveCharPosition = cur.Char
		if ctx.Editor != nil && ctx.Editor.ActiveWindow() != nil {
			ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, cur)
		}
		SelectionExtend(ctx.Buf, cur)
		break
	}
}

var LastSearchPattern string

func CmdSearchNext(ctx Context) {
	SearchNext(ctx, LastSearchPattern)
}

func CmdSearchPrev(ctx Context) {
	SearchPrev(ctx, LastSearchPattern)
}
