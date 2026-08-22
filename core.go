package wig

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const minVisibleLines = 5

func TextInsert(buf *Buffer, line *Element[Line], pos int, text string) {
	if buf == nil || line == nil || text == "" {
		return
	}
	buf.Dirty = true
	sline := CursorNumByLine(buf, line)

	// Normalize CRLF and CR to LF
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	if pos < 0 {
		pos = 0
	}
	size := len(line.Value)
	if size > 0 && pos >= size {
		pos = size - 1
	}

	prefix := line.Value[:pos]
	suffix := line.Value[pos:]

	lines := strings.Split(text, "\n")
	numLines := len(lines)

	event := EventTextChange{
		Buf:   buf,
		Start: Position{Line: sline, Char: pos},
		End:   Position{Line: sline, Char: pos},
		Text:  text,
	}

	if numLines == 1 {
		line.Value = slices.Concat(prefix, []rune(lines[0]), suffix)
		event.NewEnd = Position{Line: sline, Char: pos + utf8.RuneCountInString(lines[0])}
	} else {
		// First line receives prefix + lines[0] + newline
		line.Value = slices.Concat(prefix, []rune(lines[0]+"\n"))
		curr := line

		// Intermediate lines receive lines[i] + newline
		for i := 1; i < numLines-1; i++ {
			curr = buf.Lines.insertValueAfter([]rune(lines[i]+"\n"), curr)
		}

		// Last line receives lines[numLines-1] + suffix
		buf.Lines.insertValueAfter(slices.Concat([]rune(lines[numLines-1]), suffix), curr)

		event.NewEnd = Position{
			Line: sline + numLines - 1,
			Char: utf8.RuneCountInString(lines[numLines-1]),
		}
	}

	if EditorInst != nil && EditorInst.Events != nil {
		EditorInst.Events.Broadcast(event)
	}
}

func TextDelete(buf *Buffer, selection *Selection) {
	buf.Dirty = true
	defer func() {
		if buf.Lines.Len == 1 && len(buf.Lines.First().Value) == 0 {
			buf.Lines.First().Value = []rune{'\n'}
		}
	}()

	sel := SelectionNormalize(selection)
	lineStart := CursorLineByNum(buf, sel.Start.Line)
	lineEnd := CursorLineByNum(buf, sel.End.Line)
	sel.End.Char--
	oldText := SelectionToString(buf, &sel)
	sel.End.Char++

	// if request is to delete more chars then len(end) - we must connect next line
	// since we delete "\n"
	if sel.End.Char >= len(lineEnd.Value) {
		tmpLine := lineEnd.Next()
		if tmpLine != nil {
			sel.End.Line++
			sel.End.Char = 0
			lineEnd = tmpLine
		}
	}

	if lineStart != lineEnd {
		for lineStart.Next() != lineEnd {
			if lineStart.Next() == nil {
				break
			}
			buf.Lines.Remove(lineStart.Next())
		}
		buf.Lines.Remove(lineEnd)
	}

	start := max(0, sel.Start.Char)
	end := min(utf8.RuneCountInString(lineEnd.Value.String()), sel.End.Char)

	lineStart.Value = slices.Concat(lineStart.Value[:start], lineEnd.Value[end:])

	event := EventTextChange{
		Buf:     buf,
		Start:   Position{Line: sel.Start.Line, Char: sel.Start.Char},
		End:     Position{Line: sel.End.Line, Char: sel.End.Char},
		Text:    "",
		OldText: oldText,
	}

	EditorInst.Events.Broadcast(event)
}

func CmdJoinNextLine(ctx Context) {
	CmdGotoLineEnd(ctx)
	cur := ContextCursorGet(ctx)

	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}

	line := CursorLine(ctx.Buf, cur)
	TextDelete(ctx.Buf, &Selection{
		Start: Cursor{Line: cur.Line, Char: len(line.Value) - 1},
		End:   Cursor{Line: cur.Line, Char: len(line.Value)},
	})
}

func CmdReplaceChar(ctx Context) func(Context) {
	return func(ctx Context) {
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		cur := ContextCursorGet(ctx)
		line := CursorLine(ctx.Buf, cur)
		ctx.Buf.Selection = &Selection{
			Start: *cur,
			End:   *cur,
		}
		SelectionDelete(ctx)
		TextInsert(ctx.Buf, line, cur.Char, ctx.Char)

		// Save closure for dot repeat
		ctx.Editor.LastRepeatableFn = CmdReplaceChar(ctx)
	}
}

func CmdDeleteCharForward(ctx Context) {
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}

	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	if len(line.Value) <= 1 {
		return
	}

	if cur.Char >= len(line.Value)-1 {
		CmdCursorLeft(ctx)
	}

	count := max(int(ctx.Count), 1)
	endChar := min(cur.Char+count-1, len(line.Value)-2)

	ctx.Buf.Selection = &Selection{
		Start: *cur,
		End:   Cursor{Line: cur.Line, Char: endChar},
	}

	yankSave(ctx)
	SelectionDelete(ctx)

	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdDeleteCharForward(c)
	}
}

func CmdDeleteCharBackward(ctx Context) {
	cur := ContextCursorGet(ctx)
	if cur.Char == 0 {
		return
	}
	CmdCursorLeft(ctx)
	CmdDeleteCharForward(ctx)
}

func CmdAppendLine(ctx Context) {
	CmdGotoLineEnd(ctx)
	CmdEnterInsertModeAppend(ctx)
}

func CmdLineOpenBelow(ctx Context) {
	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	CmdAppendLine(ctx)
	TextInsert(ctx.Buf, line, len(line.Value)-1, "\n")
	CmdCursorLineDown(ctx)
	CmdCursorBeginningOfTheLine(ctx)
	indent(ctx)
}

func CmdLineOpenAbove(ctx Context) {
	cur := ContextCursorGet(ctx)
	if cur.Line == 0 {
		CmdEnterInsertMode(ctx)
		TextInsert(ctx.Buf, CursorLine(ctx.Buf, cur), 0, "\n")
		CmdCursorBeginningOfTheLine(ctx)
		return
	}
	CmdCursorLineUp(ctx)
	CmdLineOpenBelow(ctx)
}

func CmdDeleteLine(ctx Context) {
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}

	if ctx.Count == 0 {
		ctx.Count = 1
	}

	CmdVisualLineMode(ctx)
	ctx.Buf.Selection.End.Line = min(
		ctx.Buf.Lines.Len-1,
		ctx.Buf.Selection.Start.Line+int(ctx.Count)-1,
	)
	ctx.Buf.Selection.End.Char = len(CursorLineByNum(ctx.Buf, ctx.Buf.Selection.End.Line).Value) - 1
	yankSave(ctx)
	SelectionDelete(ctx)
	CmdNormalMode(ctx)

	// Register for command repeat, capturing the count so `.` repeats
	// with the same count (e.g. `2dd` → `.` deletes 2 lines, not 1).
	// A count before `.` (e.g. `3.`) overrides the saved count, like Vim.
	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdDeleteLine(c)
	}
}

func CmdDeleteWord(ctx Context) {
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}
	count := max(int(ctx.Count), 1)

	var deletedText string
	for i := 0; i < count; i++ {
		cur := ContextCursorGet(ctx)
		_, end := TextObjectWord(ctx, false)
		ctx.Buf.Selection = &Selection{
			Start: *cur,
			End:   Cursor{Line: cur.Line, Char: end},
		}
		deletedText += SelectionToString(ctx.Buf, ctx.Buf.Selection)
		SelectionDelete(ctx)
	}

	y := yank{val: deletedText}
	if ctx.Editor.Yanks.Len == 0 || ctx.Editor.Yanks.Last().Value != y {
		ctx.Editor.Yanks.PushBack(y)
	}

	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdDeleteWord(c)
	}
}

func CmdChangeWord(ctx Context) {
	cur := ContextCursorGet(ctx)
	_, end := TextObjectWord(ctx, false)
	ctx.Buf.Selection = &Selection{
		Start: *cur,
		End:   Cursor{Line: cur.Line, Char: end},
	}
	CmdEnterInsertMode(ctx)
	yankSave(ctx)
	SelectionDelete(ctx)

	ctx.Editor.LastRepeatableFn = CmdChangeWord
}

func CmdChangeWORD(ctx Context) {
	cur := ContextCursorGet(ctx)
	start, end := TextObjectWord(ctx, true)
	cur.Char = start
	ctx.Buf.Selection = &Selection{
		Start: *cur,
		End:   Cursor{Line: cur.Line, Char: end},
	}
	CmdEnterInsertMode(ctx)
	yankSave(ctx)
	SelectionDelete(ctx)
}

func CmdChangeTo(_ Context) func(Context) {
	return func(ctx Context) {
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardToChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		CmdEnterInsertMode(ctx)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdChangeBefore(_ Context) func(Context) {
	return func(ctx Context) {
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardBeforeChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		CmdEnterInsertMode(ctx)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdChangeLine(ctx Context) {
	CmdCursorFirstNonBlank(ctx)
	CmdChangeEndOfLine(ctx)
}

func CmdChangeEndOfLine(ctx Context) {
	ctx.Char = "\n"
	CmdChangeBefore(ctx)(ctx)
}

func CmdDeleteEndOfLine(ctx Context) {
	ctx.Char = "\n"
	CmdChangeBefore(ctx)(ctx)
	CmdNormalMode(ctx)
}

func CmdDeleteTo(_ Context) func(Context) {
	return func(ctx Context) {
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardToChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdDeleteBefore(ctx Context) func(Context) {
	return func(ctx Context) {
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardBeforeChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdSelectionChange(ctx Context) {
	CmdEnterInsertMode(ctx)
	yankSave(ctx)
	SelectionDelete(ctx)
}

// ToggleCommentRange toggles comments across lines [startLine, endLine] inclusive.
func ToggleCommentRange(ctx Context, startLine, endLine int) {
	if ctx.Buf == nil {
		return
	}
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}
	defer CmdNormalMode(ctx)

	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= ctx.Buf.Lines.Len {
		endLine = ctx.Buf.Lines.Len - 1
	}

	comment := GetCommentToken(ctx.Buf)

	cmComment := func(line *Element[Line], insertCol int) {
		TextInsert(ctx.Buf, line, insertCol, comment+" ")
	}

	cmUncomment := func(line *Element[Line]) {
		str := string(line.Value)
		trimmed := strings.TrimLeftFunc(str, unicode.IsSpace)
		leadLen := len([]rune(str[:len(str)-len(trimmed)]))
		lineNum := CursorNumByLine(ctx.Buf, line)

		prefixToDel := comment
		if strings.HasPrefix(trimmed, comment+" ") {
			prefixToDel = comment + " "
		}

		TextDelete(ctx.Buf, &Selection{
			Start: Cursor{Line: lineNum, Char: leadLen},
			End:   Cursor{Line: lineNum, Char: leadLen + len([]rune(prefixToDel))},
		})
	}

	isLineCommented := func(line *Element[Line]) bool {
		trimmed := strings.TrimSpace(string(line.Value))
		return strings.HasPrefix(trimmed, comment)
	}

	lineStart := CursorLineByNum(ctx.Buf, startLine)
	count := endLine - startLine

	lines := make([]*Element[Line], 0, count+1)
	curr := lineStart
	for i := 0; i <= count && curr != nil; i++ {
		lines = append(lines, curr)
		curr = curr.Next()
	}

	allCommented := true
	hasNonEmpty := false

	for _, l := range lines {
		if l.Value.IsEmpty() {
			continue
		}
		hasNonEmpty = true
		if !isLineCommented(l) {
			allCommented = false
			break
		}
	}

	if !hasNonEmpty {
		return
	}

	for _, l := range lines {
		if l.Value.IsEmpty() {
			continue
		}
		if allCommented {
			cmUncomment(l)
		} else {
			spacePos := 0
			for i, c := range l.Value {
				if !unicode.IsSpace(c) {
					spacePos = i
					break
				}
			}
			cmComment(l, spacePos)
		}
	}
}

func CmdToggleComment(ctx Context) {
	if ctx.Buf == nil {
		return
	}
	if ctx.Buf.Selection != nil {
		selection := SelectionNormalize(ctx.Buf.Selection)
		ToggleCommentRange(ctx, selection.Start.Line, selection.End.Line)
		ctx.Editor.LastRepeatableFn = CmdToggleComment
		return
	}

	cur := ContextCursorGet(ctx)
	count := max(int(ctx.Count), 1)
	ToggleCommentRange(ctx, cur.Line, cur.Line+count-1)
	ctx.Editor.LastRepeatableFn = CmdToggleComment
}

// CmdCommentLine comments the current line (or count lines) — `gcc` mapping.
func CmdCommentLine(ctx Context) {
	cur := ContextCursorGet(ctx)
	count := max(int(ctx.Count), 1)
	ToggleCommentRange(ctx, cur.Line, cur.Line+count-1)
	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdCommentLine(c)
	}
}

// CmdCommentLineDown comments current line and count lines below — `gcj` mapping.
func CmdCommentLineDown(ctx Context) {
	cur := ContextCursorGet(ctx)
	count := max(int(ctx.Count), 1)
	ToggleCommentRange(ctx, cur.Line, cur.Line+count)
	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdCommentLineDown(c)
	}
}

// CmdCommentLineUp comments current line and count lines above — `gck` mapping.
func CmdCommentLineUp(ctx Context) {
	cur := ContextCursorGet(ctx)
	count := max(int(ctx.Count), 1)
	ToggleCommentRange(ctx, cur.Line-count, cur.Line)
	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdCommentLineUp(c)
	}
}

// CmdCommentEndOfLine comments the current line — `gc$` mapping.
func CmdCommentEndOfLine(ctx Context) {
	cur := ContextCursorGet(ctx)
	ToggleCommentRange(ctx, cur.Line, cur.Line)
	ctx.Editor.LastRepeatableFn = CmdCommentEndOfLine
}

// CmdCommentWord comments the current line — `gcw` mapping.
func CmdCommentWord(ctx Context) {
	cur := ContextCursorGet(ctx)
	ToggleCommentRange(ctx, cur.Line, cur.Line)
	ctx.Editor.LastRepeatableFn = CmdCommentWord
}

// CmdCommentEndOfFile comments from current line to end of file — `gcG` mapping.
func CmdCommentEndOfFile(ctx Context) {
	cur := ContextCursorGet(ctx)
	ToggleCommentRange(ctx, cur.Line, ctx.Buf.Lines.Len-1)
	ctx.Editor.LastRepeatableFn = CmdCommentEndOfFile
}

// CmdCommentStartOfFile comments from line 0 to current line — `gcgg` mapping.
func CmdCommentStartOfFile(ctx Context) {
	cur := ContextCursorGet(ctx)
	ToggleCommentRange(ctx, 0, cur.Line)
	ctx.Editor.LastRepeatableFn = CmdCommentStartOfFile
}

func getParagraphRange(buf *Buffer, curLine int, includeTrailingBlank bool) (startLine, endLine int) {
	if buf == nil || buf.Lines.Len == 0 {
		return curLine, curLine
	}
	startLine = curLine
	for startLine > 0 {
		prev := CursorLineByNum(buf, startLine-1)
		if prev == nil || prev.Value.IsEmpty() {
			break
		}
		startLine--
	}

	endLine = curLine
	for endLine < buf.Lines.Len-1 {
		next := CursorLineByNum(buf, endLine+1)
		if next == nil || next.Value.IsEmpty() {
			if includeTrailingBlank && next != nil && next.Value.IsEmpty() {
				endLine++
			}
			break
		}
		endLine++
	}
	return startLine, endLine
}

// CmdCommentParagraph comments the surrounding paragraph — `gcip` mapping.
func CmdCommentParagraph(ctx Context) {
	cur := ContextCursorGet(ctx)
	start, end := getParagraphRange(ctx.Buf, cur.Line, false)
	ToggleCommentRange(ctx, start, end)
	ctx.Editor.LastRepeatableFn = CmdCommentParagraph
}

// CmdCommentAroundParagraph comments the surrounding paragraph including trailing blank — `gcap` mapping.
func CmdCommentAroundParagraph(ctx Context) {
	cur := ContextCursorGet(ctx)
	start, end := getParagraphRange(ctx.Buf, cur.Line, true)
	ToggleCommentRange(ctx, start, end)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundParagraph
}

// CmdCommentInside handles `gci{target}` (e.g. `gcip`, `gciw`).
func CmdCommentInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])
		if ch == 'p' {
			CmdCommentParagraph(ctx)
			return
		}
		if ch == 'w' || ch == 'W' {
			CmdCommentWord(ctx)
			return
		}

		var sel *Selection
		var found bool
		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, false)
		} else {
			found, sel = TextObjectBlock(ctx, ch, false)
		}

		if !found || sel == nil {
			return
		}

		norm := SelectionNormalize(sel)
		ToggleCommentRange(ctx, norm.Start.Line, norm.End.Line)
	}
}

// CmdCommentAround handles `gca{target}` (e.g. `gcap`, `gcaw`).
func CmdCommentAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])
		if ch == 'p' {
			CmdCommentAroundParagraph(ctx)
			return
		}
		if ch == 'w' || ch == 'W' {
			CmdCommentWord(ctx)
			return
		}

		var sel *Selection
		var found bool
		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, true)
		} else {
			found, sel = TextObjectBlock(ctx, ch, true)
		}

		if !found || sel == nil {
			return
		}

		norm := SelectionNormalize(sel)
		ToggleCommentRange(ctx, norm.Start.Line, norm.End.Line)
	}
}

func CmdSelectionDelete(ctx Context) {
	defer CmdNormalMode(ctx)
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}
	yankSave(ctx)
	SelectionDelete(ctx)

	ctx.Editor.LastRepeatableFn = CmdSelectionDelete
}

func CmdSaveFile(ctx Context) {
	if ctx.Char != "" && (ctx.Buf.FilePath == "" || ctx.Buf.FilePath == "[No Name]") {
		ctx.Buf.FilePath = ctx.Char
	}
	if ctx.Buf.FilePath == "" || ctx.Buf.FilePath == "[No Name]" {
		ctx.Editor.EchoMessage("No file name")
		return
	}
	err := ctx.Buf.Save()
	if err != nil {
		ctx.Editor.LogMessage(err.Error())
		ctx.Editor.EchoMessage(err.Error())
		return
	}
	ctx.Buf.Dirty = false

	if ctx.Editor.Config.FormatOnSave {
		if def, ok := AllCommands["CmdFormatBuffer"]; ok {
			if fn, ok := def.Fn.(func(Context)); ok {
				fn(ctx)
				_ = ctx.Buf.Save()
				ctx.Editor.Lsp.DidClose(ctx.Buf)
				ctx.Editor.Lsp.DidOpen(ctx.Buf)
				if ctx.Buf.Highlighter != nil {
					ctx.Buf.Highlighter.Build()
				}
			}
		}
	}

	// Mark saved state *after* any format-on-save transaction has been pushed.
	ctx.Buf.UndoRedo.SavedAtPosition = ctx.Buf.UndoRedo.Position
	ctx.Editor.Lsp.DidSave(ctx.Buf)
}

func CmdKillBuffer(ctx Context) {
	if len(ctx.Editor.Buffers) == 0 {
		return
	}

	buf := ctx.Buf

	// Save the cursor position of the buffer being killed to position.toml
	if buf.FilePath != "" && !strings.HasPrefix(buf.FilePath, "[") {
		posCache := LoadPositionCache()
		cur := WindowCursorGet(ctx.Editor.ActiveWindow(), buf)
		posCache.Files[buf.FilePath] = PositionEntry{
			Line:      cur.Line,
			OpenCount: buf.OpenCount,
			Timestamp: time.Now().Unix(),
		}
		posCache.Save()
	}

	ctx.Editor.Lsp.DidClose(buf)

	// Jump back in history
	CmdJumpBack(ctx)

	// If we are still on the buffer we are trying to delete (e.g. only internal buffers left)
	if ctx.Editor.ActiveWindow().Buffer() == buf {
		if len(ctx.Editor.Buffers) > 1 {
			for _, b := range ctx.Editor.Buffers {
				if b != buf {
					ctx.Buf = b
					ctx.Editor.ActiveWindow().VisitBuffer(ctx)
					break
				}
			}
		} else {
			// No other buffers left, create a new empty one
			CmdNewBuffer(ctx)
		}
	}

	ctx.Editor.Buffers = slices.DeleteFunc(ctx.Editor.Buffers, func(b *Buffer) bool {
		if b != buf {
			return false
		}

		{
			l := buf.Lines.First()
			for l != nil {
				next := l.Next()
				l.Value = nil
				buf.Lines.Remove(l)
				l = next
			}
			buf.Selection = nil
			buf.Highlighter = nil
			buf.UndoRedo = nil
			buf.Tx = nil
			buf.KeyHandler = nil
		}

		ctx.Editor.Windows = slices.DeleteFunc(ctx.Editor.Windows, func(win *Window) bool {
			if len(ctx.Editor.Windows) == 1 {
				return false
			}
			if win.buf == b {
				return true
			}
			return false
		})

		return true
	})
}

func CmdNewBuffer(ctx Context) {
	buf := NewBuffer()
	buf.FilePath = "[No Name]"
	ctx.Editor.Buffers = append(ctx.Editor.Buffers, buf)
	ctx.Editor.ActiveWindow().ShowBuffer(buf)
}

func CmdIndentOrComplete(ctx Context) {
	ctx.Editor.Lsp.Completion(ctx.Buf)
}

func CmdChangeInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		if ch == 'w' {
			CmdChangeWord(ctx)
			return
		}
		if ch == 'W' {
			CmdChangeWORD(ctx)
			return
		}

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, false)
		} else {
			found, sel = TextObjectBlock(ctx, ch, false)
		}

		if !found {
			return
		}
		if sel == nil {
			CmdEnterInsertMode(ctx)
			return
		}

		ctx.Buf.Selection = sel
		CmdEnterInsertMode(ctx)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdChangeAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		if ch == 'w' {
			CmdChangeWORD(ctx)
			return
		}

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, true)
		} else {
			found, sel = TextObjectBlock(ctx, ch, true)
		}

		if !found || sel == nil {
			return
		}

		ctx.Buf.Selection = sel
		CmdEnterInsertMode(ctx)
		yankSave(ctx)
		SelectionDelete(ctx)
	}
}

func CmdDeleteInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		if ch == 'w' {
			CmdDeleteWord(ctx)
			return
		}

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, false)
		} else {
			found, sel = TextObjectBlock(ctx, ch, false)
		}

		if !found || sel == nil {
			return
		}

		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		ctx.Buf.Selection = sel
		yankSave(ctx)
		SelectionDelete(ctx)
		CmdNormalMode(ctx)
	}
}

func CmdDeleteAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		if ch == 'w' {
			CmdDeleteWord(ctx)
			return
		}

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, true)
		} else {
			found, sel = TextObjectBlock(ctx, ch, true)
		}

		if !found || sel == nil {
			return
		}

		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		ctx.Buf.Selection = sel
		yankSave(ctx)
		SelectionDelete(ctx)
		CmdNormalMode(ctx)
	}
}

func CmdYankInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, false)
		} else {
			found, sel = TextObjectBlock(ctx, ch, false)
		}

		if !found || sel == nil {
			return
		}

		ctx.Buf.Selection = sel
		yankSave(ctx)
		ctx.Buf.Selection = nil
	}
}

func CmdYankAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		ch := rune(ctx.Char[0])

		var sel *Selection
		var found bool

		if ch == '\'' || ch == '"' || ch == '`' {
			found, sel = TextObjectQuotes(ctx, ch, true)
		} else {
			found, sel = TextObjectBlock(ctx, ch, true)
		}

		if !found || sel == nil {
			return
		}

		ctx.Buf.Selection = sel
		yankSave(ctx)
		ctx.Buf.Selection = nil
	}
}

func CmdUndo(ctx Context) {
	ctx.Buf.UndoRedo.Undo()
	EditorInst.Events.Broadcast(EventBufferReloaded{Buf: ctx.Buf})
}

func CmdRedo(ctx Context) {
	ctx.Buf.UndoRedo.Redo()
	EditorInst.Events.Broadcast(EventBufferReloaded{Buf: ctx.Buf})
}

func CmdEnterInsertMode(ctx Context) {
	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	if line == nil {
		return
	}
	ctx.Buf.TxStart()
	setBufferMode(ctx, MODE_INSERT)
}

func CmdEnterInsertModeAppend(ctx Context) {
	CmdCursorRight(ctx)
	CmdEnterInsertMode(ctx)
}

func CmdVisualMode(ctx Context) {
	cur := ContextCursorGet(ctx)
	SelectionStart(ctx.Buf, cur)
	setBufferMode(ctx, MODE_VISUAL)
}

func CmdVisualBlockMode(ctx Context) {
	cur := ContextCursorGet(ctx)
	SelectionStart(ctx.Buf, cur)
	setBufferMode(ctx, MODE_VISUAL_BLOCK)
}

func CmdVisualBlockInsert(ctx Context) {
	cur := ContextCursorGet(ctx)
	if ctx.Buf.Selection == nil {
		return
	}
	sel := SelectionNormalize(ctx.Buf.Selection)

	cur.Line = sel.Start.Line
	cur.Char = sel.Start.Char
	cur.PreserveCharPosition = cur.Char

	ctx.Buf.VisualBlockInsert = &VisualBlockInsertState{
		StartLine: sel.Start.Line,
		EndLine:   sel.End.Line,
		Char:      sel.Start.Char,
	}

	ctx.Buf.Selection = nil
	ctx.Buf.TxStart()
	setBufferMode(ctx, MODE_INSERT)
}

func CmdExitInsertMode(ctx Context) {
	CmdNormalMode(ctx)
}

func CmdNormalMode(ctx Context) {
	if ctx.Buf.Mode() == MODE_INSERT {
		cur := ContextCursorGet(ctx)
		line := CursorLine(ctx.Buf, cur)

		if ctx.Buf.VisualBlockInsert != nil {
			vbi := ctx.Buf.VisualBlockInsert
			ctx.Buf.VisualBlockInsert = nil

			endChar := cur.Char
			if endChar > len(line.Value) {
				endChar = len(line.Value) - 1
			}

			if vbi.Char < endChar {
				insertedText := string(line.Value[vbi.Char:endChar])
				for i := vbi.StartLine + 1; i <= vbi.EndLine; i++ {
					l := CursorLineByNum(ctx.Buf, i)
					if l != nil {
						TextInsert(ctx.Buf, l, vbi.Char, insertedText)
					}
				}
			}

			cur.Line = vbi.StartLine
			cur.Char = vbi.Char
		}

		CmdCursorLeft(ctx)
		if cur.Char >= len(line.Value) {
			CmdGotoLineEnd(ctx)
		}
	}

	ctx.Buf.TxEnd()
	setBufferMode(ctx, MODE_NORMAL)
	ctx.Buf.Selection = nil
}

func CmdVisualLineMode(ctx Context) {
	cur := ContextCursorGet(ctx)
	line := CursorLine(ctx.Buf, cur)
	SelectionStart(ctx.Buf, cur)
	ctx.Buf.Selection.Start.Char = 0
	ctx.Buf.Selection.End.Char = len(line.Value) - 1
	setBufferMode(ctx, MODE_VISUAL_LINE)
}

func setBufferMode(ctx Context, newMode Mode) {
	// ctx.Editor.Events.Broadcast(EventBufferModeChange{
	// Buf:     ctx.Buf,
	// OldMode: ctx.Buf.Mode(),
	// NewMode: newMode,
	// })
	ctx.Buf.SetMode(newMode)
}

func CmdMacroRecord(ctx Context) func(Context) {
	if ctx.Editor.Keys.Macros.Recording() {
		ctx.Editor.Keys.Macros.Stop()
		ctx.Editor.Keys.resetState()
		return nil
	}

	return func(ctx Context) {
		ctx.Editor.Keys.Macros.Start(ctx.Char)
	}
}

func CmdMacroPlay(ctx Context) func(Context) {
	return func(ctx Context) {
		ctx.Editor.Keys.resetState()

		reg := ctx.Char
		count := max(ctx.Count, 1)
		for i := uint32(0); i < count; i++ {
			ctx.Editor.Keys.Macros.Play(reg)
		}
	}
}

func CmdMacroRepeat(ctx Context) {
	if ctx.Editor.LastRepeatableFn != nil {
		if !ctx.Editor.Keys.Macros.IsPlaying() {
			ctx.Editor.LastRepeatableFn(ctx)
		}
	}
}

func CmdAutocompleteTrigger(ctx Context) {
	ctx.Editor.AutocompleteTrigger(ctx)
}

func CmdBufferNext(ctx Context) {
	active := ctx.Editor.ActiveBuffer()
	buffers := ctx.Editor.Buffers
	if len(buffers) <= 1 {
		return
	}
	idx := 0
	for i, b := range buffers {
		if b == active {
			idx = i
			break
		}
	}
	for i := 1; i <= len(buffers); i++ {
		next := buffers[(idx+i)%len(buffers)]
		if !strings.HasPrefix(next.GetName(), "[") {
			ctx.Buf = next
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
			return
		}
	}
}

func CmdBufferPrev(ctx Context) {
	active := ctx.Editor.ActiveBuffer()
	buffers := ctx.Editor.Buffers
	if len(buffers) <= 1 {
		return
	}
	idx := 0
	for i, b := range buffers {
		if b == active {
			idx = i
			break
		}
	}
	for i := 1; i <= len(buffers); i++ {
		prev := buffers[(idx-i+len(buffers))%len(buffers)]
		if !strings.HasPrefix(prev.GetName(), "[") {
			ctx.Buf = prev
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
			return
		}
	}
}

func CmdBufferLast(ctx Context) {
	buffers := ctx.Editor.Buffers
	if len(buffers) == 0 {
		return
	}
	ctx.Buf = buffers[len(buffers)-1]
	ctx.Editor.ActiveWindow().VisitBuffer(ctx)
}

func CmdPaste(ctx Context) {
	panic(1)
}

// CmdSetMark waits for a character input and sets a mark at the current cursor position.
func CmdSetMark(ctx Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		r := rune(ctx.Char[0])
		if ctx.Win.Marks == nil {
			ctx.Win.Marks = make(map[rune]Cursor)
		}
		cur := ContextCursorGet(ctx)
		ctx.Win.Marks[r] = *cur
		ctx.Editor.EchoMessage("Mark '" + string(r) + "' set")
	}
}

// CmdGotoMark opens the marks popup legend.
func CmdGotoMark(ctx Context) {
	if MarksPopupFactory != nil {
		MarksPopupFactory(ctx, ctx.Win.Marks)
	}
}

// CmdDummyNA is a no-op command used to disable or override keybindings in config.toml
// by mapping them to this function (e.g. `x = "CmdDummyNA"`).
func CmdDummyNA(ctx Context) {
	// No operation
}
