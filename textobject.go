package wig

import "strings"

// WordOrSelectionUnderCursor extracts the active selection or the word under the cursor.
// If a selection is active, it switches the buffer back to normal mode.
func WordOrSelectionUnderCursor(ctx Context) (string, bool) {
	if ctx.Buf == nil {
		return "", false
	}

	if ctx.Buf.Selection != nil {
		word := SelectionToString(ctx.Buf, ctx.Buf.Selection)
		CmdNormalMode(ctx)
		word = strings.TrimSpace(word)
		return word, word != ""
	}

	cur := ContextCursorGet(ctx)
	if cur == nil {
		return "", false
	}

	line := CursorLine(ctx.Buf, cur)
	if line == nil || line.Value.IsEmpty() {
		return "", false
	}

	if CursorChClass(ctx.Buf, cur) == 0 {
		CmdBackwardWord(ctx)
	}

	start, end := TextObjectWord(ctx, true)
	if end+1 > start {
		line = CursorLine(ctx.Buf, cur)
		if line != nil {
			word := strings.TrimSpace(string(line.Value.Range(start, end+1)))
			return word, word != ""
		}
	}

	return "", false
}

func TextObjectWord(ctx Context, bigword bool) (start, end int) {
	cur := ContextCursorGet(ctx)
	start = cur.Char
	end = start

	line := CursorLine(ctx.Buf, cur)
	if line == nil {
		return start, end
	}
	cls := CursorChClass(ctx.Buf, cur)

	if bigword {
		for start > 0 {
			if line.Value.IsEmpty() {
				break
			}
			if getChClass(line.Value[start-1]) == cls {
				start--
			} else {
				break
			}
		}
	}

	end = start

	for i, r := range line.Value {
		if i < start {
			continue
		}
		if getChClass(r) == cls {
			end = i
			continue
		}

		break
	}

	return start, end
}

// TextObjectBlock finds the enclosing bracket pair for the cursor position.
// Supports (), {}, [], <>. Scans backward for the opening bracket, then
// forward for the matching close. Handles nesting and multi-line spans.
// If include is true, the selection includes the brackets; otherwise it's
// the content inside them. Returns nil selection for empty brackets like ().
func TextObjectBlock(ctx Context, ch rune, include bool) (found bool, sel *Selection) {
	cur := ContextCursorGet(ctx)

	openClose := map[rune]rune{
		'(': ')',
		'{': '}',
		'[': ']',
		'<': '>',
	}

	closeCh, ok := openClose[ch]
	if !ok {
		for k, v := range openClose {
			if v == ch {
				ch = k
				closeCh = v
				ok = true
				break
			}
		}
		if !ok {
			return false, nil
		}
	}

	// If cursor is on the opening bracket, scan forward directly
	curChar := CursorChar(ctx.Buf, cur)
	if curChar == ch {
		return findCloseBracket(ctx, *cur, ch, closeCh, include)
	}

	// Scan backward to find enclosing opening bracket
	scanCur := *cur
	depth := 0

	for {
		r := CursorChar(ctx.Buf, &scanCur)
		if r == closeCh {
			depth++
		} else if r == ch {
			if depth == 0 {
				return findCloseBracket(ctx, scanCur, ch, closeCh, include)
			}
			depth--
		}
		if !CursorDec(ctx.Buf, &scanCur) {
			break
		}
	}

	return false, nil
}

func findCloseBracket(ctx Context, openPos Cursor, openCh, closeCh rune, include bool) (found bool, sel *Selection) {
	scanCur := openPos
	depth := 1

	if !CursorInc(ctx.Buf, &scanCur) {
		return false, nil
	}

	for {
		r := CursorChar(ctx.Buf, &scanCur)
		if r == openCh {
			depth++
		} else if r == closeCh {
			depth--
			if depth == 0 {
				if include {
					return true, &Selection{
						Start: openPos,
						End:   scanCur,
					}
				}
				innerStart := openPos
				innerStart.Char++
				innerEnd := scanCur
				innerEnd.Char--
				if openPos.Line == scanCur.Line && innerStart.Char > innerEnd.Char {
					return true, nil
				}
				return true, &Selection{
					Start: innerStart,
					End:   innerEnd,
				}
			}
		}
		if !CursorInc(ctx.Buf, &scanCur) {
			break
		}
	}

	return false, nil
}

// TextObjectQuotes finds a matching quote pair on the current line.
// Supports ', ", `. Pairs are matched left-to-right (first pair, second pair, etc.).
// If the cursor is inside a pair, that pair is used. If before the first
// quote, the first pair is used.
func TextObjectFunction(ctx Context, include bool) (found bool, sel *Selection) {
	hl, ok := ctx.Buf.Highlighter.(*TreeSitterHighlighter)
	if !ok || hl == nil {
		return false, nil
	}
	cur := ContextCursorGet(ctx)
	startLine, endLine, ok := hl.FunctionRange(cur.Line)
	if !ok {
		return false, nil
	}

	startChar := 0
	endLineNode := CursorLineByNum(ctx.Buf, endLine)
	endChar := len(endLineNode.Value) - 1
	if endChar < 0 {
		endChar = 0
	}

	if include {
		if endLine+1 < ctx.Buf.Lines.Len {
			nextLine := CursorLineByNum(ctx.Buf, endLine+1)
			if nextLine.Value.IsEmpty() {
				endLine++
				endLineNode = nextLine
				endChar = len(endLineNode.Value) - 1
				if endChar < 0 {
					endChar = 0
				}
			}
		}
	} else {
		startLineNode := CursorLineByNum(ctx.Buf, startLine)
		if len(startLineNode.Value) > 1 {
			startChar = len(startLineNode.Value) - 1
		}
	}

	return true, &Selection{
		Start: Cursor{Line: startLine, Char: startChar},
		End:   Cursor{Line: endLine, Char: endChar},
	}
}

func TextObjectQuotes(ctx Context, ch rune, include bool) (found bool, sel *Selection) {
	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	if line == nil {
		return false, nil
	}

	lineRunes := line.Value
	charIdx := cur.Char
	if charIdx >= len(lineRunes) {
		charIdx = len(lineRunes) - 1
	}

	var positions []int
	for i, r := range lineRunes {
		if r == ch {
			positions = append(positions, i)
		}
	}

	if len(positions) < 2 {
		return false, nil
	}

	for i := 0; i < len(positions)-1; i += 2 {
		start := positions[i]
		end := positions[i+1]

		if charIdx >= start && charIdx <= end {
			if include {
				return true, &Selection{
					Start: Cursor{Line: cur.Line, Char: start},
					End:   Cursor{Line: cur.Line, Char: end},
				}
			}
			if start+1 > end-1 {
				return true, nil
			}
			return true, &Selection{
				Start: Cursor{Line: cur.Line, Char: start + 1},
				End:   Cursor{Line: cur.Line, Char: end - 1},
			}
		}
	}

	// Cursor before first quote — use first pair
	if charIdx < positions[0] {
		start := positions[0]
		end := positions[1]
		if include {
			return true, &Selection{
				Start: Cursor{Line: cur.Line, Char: start},
				End:   Cursor{Line: cur.Line, Char: end},
			}
		}
		if start+1 > end-1 {
			return true, nil
		}
		return true, &Selection{
			Start: Cursor{Line: cur.Line, Char: start + 1},
			End:   Cursor{Line: cur.Line, Char: end - 1},
		}
	}

	return false, nil
}
