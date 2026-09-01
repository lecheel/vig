package wig

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const minVisibleLines = 5

// LastInsertedText tracks the last inserted text for the '.' register
var LastInsertedText string

// OnSaveHook is invoked after a buffer is successfully saved to disk.
// External packages (e.g. commands) that cannot import wig directly set this
// to receive save notifications — used by the ctagd daemon client to push
// `saved` events without blocking the editor.
var OnSaveHook func(Context)

// GitBranchProvider, when set, returns the current git branch for the
// repository containing buf (and whether one was found). Populated by the
// commands package, which owns git subprocess calls; ui reads this via wig
// instead of importing commands directly, since commands already imports
// ui (see commands/git_view.go, git_hunk.go) and a reverse import would
// create a cycle.
var GitBranchProvider func(buf *Buffer) (branch string, ok bool)

func TextInsert(buf *Buffer, line *Element[Line], pos int, text string) {
	if buf == nil || line == nil || text == "" {
		return
	}
	LastInsertedText = text
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
	if len(suffix) == 0 || suffix[len(suffix)-1] != '\n' {
		suffix = append(suffix, '\n')
	}

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

	if event.NewEnd.Line > sline {
		// Lines were inserted starting at sline (e.g. a multi-line paste
		// or pressing Enter). Nothing "inside" the insertion point moves;
		// everything strictly after it shifts down by the number of new
		// lines. This mirrors vim's convention of using an empty
		// [line1, line1-1] range for pure insertions.
		amount := event.NewEnd.Line - sline
		MarkAdjustInternal(buf, sline+1, sline, amount, 0)
		if EditorInst != nil && EditorInst.Marks != nil {
			for r, m := range EditorInst.Marks {
				if m.Buf == buf && m.Cursor.Line == sline && m.Cursor.Char >= pos {
					m.Cursor.Line = event.NewEnd.Line
					m.Cursor.Char = event.NewEnd.Char + (m.Cursor.Char - pos)
					EditorInst.Marks[r] = m
				}
			}
		}
	} else if insertedLen := utf8.RuneCountInString(lines[0]); insertedLen > 0 {
		// Single-line insertion: marks at/after the insertion column on
		// this line shift right by the inserted text length.
		MarkColAdjust(buf, sline, pos, insertedLen, 0)
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

	if linesDeleted := sel.End.Line - sel.Start.Line; linesDeleted > 0 {
		// The deletion spans [sel.Start.Line, sel.End.Line]; amount is
		// negative since lines are being removed, so soft marks in the
		// range collapse onto sel.Start.Line and hard marks are
		// invalidated (see MarkAdjustInternal).
		MarkAdjustInternal(buf, sel.Start.Line, sel.End.Line, -linesDeleted, 0)
	} else {
		// Pure same-line deletion never touches line numbers, so it fell
		// through the line-range tracking above entirely — marks on this
		// line still need to collapse/shift for the deleted columns.
		delStart := max(0, sel.Start.Char)
		delEnd := min(utf8.RuneCountInString(lineEnd.Value.String()), sel.End.Char)
		if delEnd > delStart {
			MarkColDelete(buf, sel.Start.Line, delStart, delEnd)
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

func CmdDeleteEndOfFile(ctx Context) {
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}
	CmdVisualLineMode(ctx)
	ctx.Buf.Selection.End.Line = ctx.Buf.Lines.Len - 1
	ctx.Buf.Selection.End.Char = len(CursorLineByNum(ctx.Buf, ctx.Buf.Lines.Len-1).Value) - 1
	yankSave(ctx)
	SelectionDelete(ctx)
	CmdNormalMode(ctx)
	ctx.Editor.LastRepeatableFn = CmdDeleteEndOfFile
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
		TextObjectComment(ctx, rune(ctx.Char[0]), false)
	}
}

// CmdCommentAround handles `gca{target}` (e.g. `gcap`, `gcaw`).
func CmdCommentAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectComment(ctx, rune(ctx.Char[0]), true)
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
	if OnSaveHook != nil {
		OnSaveHook(ctx)
	}
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

	// Remove any marks associated with the killed buffer
	if ctx.Editor.Marks != nil {
		for r, m := range ctx.Editor.Marks {
			if m.Buf == buf {
				delete(ctx.Editor.Marks, r)
			}
		}
	}

	ctx.Editor.Lsp.DidClose(buf)

	// Jump back in history
	CmdJumpBack(ctx)

	// Replace the killed buffer in all windows across all workspaces
	var replacement *Buffer
	if len(ctx.Editor.Buffers) > 1 {
		for _, b := range ctx.Editor.Buffers {
			if b != buf {
				replacement = b
				break
			}
		}
	} else {
		// No other buffers left, create a new empty one
		CmdNewBuffer(ctx)
		replacement = ctx.Editor.Buffers[0]
	}

	for i := range ctx.Editor.Workspaces {
		ws := &ctx.Editor.Workspaces[i]
		ws.Files = slices.DeleteFunc(ws.Files, func(f string) bool {
			return f == buf.FilePath
		})
		for _, win := range ws.Windows {
			if win.Buffer() == buf {
				nctx := ctx.Editor.NewContext()
				nctx.Buf = replacement
				nctx.Win = win
				win.VisitBuffer(nctx)
			}
		}
	}
	if ctx.Editor.ActiveWindow().Buffer() == buf {
		ctx.Buf = replacement
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

		for i := range ctx.Editor.Workspaces {
			ws := &ctx.Editor.Workspaces[i]
			ws.Windows = slices.DeleteFunc(ws.Windows, func(win *Window) bool {
				if len(ws.Windows) == 1 {
					return false
				}
				if win.buf == b {
					if ws.Root != nil {
						ws.Root, _ = removeLeaf(ws.Root, win)
					}
					return true
				}
				return false
			})
			if ws.Root == nil && len(ws.Windows) > 0 {
				ws.Root = leafNode(ws.Windows[0])
			}
		}

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

func TextObjectDelete(ctx Context, ch rune, include bool) {
	if ctx.Buf == nil {
		return
	}
	if ch == 'w' {
		CmdDeleteWord(ctx)
		return
	}
	if ch == 'p' {
		cur := ContextCursorGet(ctx)
		start, end := getParagraphRange(ctx.Buf, cur.Line, include)
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		CmdVisualLineMode(ctx)
		ctx.Buf.Selection.Start.Line = start
		ctx.Buf.Selection.Start.Char = 0
		ctx.Buf.Selection.End.Line = end
		ctx.Buf.Selection.End.Char = len(CursorLineByNum(ctx.Buf, end).Value) - 1
		yankSave(ctx)
		SelectionDelete(ctx)
		CmdNormalMode(ctx)
		return
	}
	var sel *Selection
	var found bool
	if ch == 'f' {
		found, sel = TextObjectFunction(ctx, include)
	} else if ch == '\'' || ch == '"' || ch == '`' {
		found, sel = TextObjectQuotes(ctx, ch, include)
	} else {
		found, sel = TextObjectBlock(ctx, ch, include)
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

func TextObjectChange(ctx Context, ch rune, include bool) {
	if ctx.Buf == nil {
		return
	}
	if ch == 'w' {
		if include {
			CmdChangeWORD(ctx)
		} else {
			CmdChangeWord(ctx)
		}
		return
	}
	if ch == 'W' {
		CmdChangeWORD(ctx)
		return
	}
	if ch == 'p' {
		cur := ContextCursorGet(ctx)
		start, end := getParagraphRange(ctx.Buf, cur.Line, include)
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		CmdVisualLineMode(ctx)
		ctx.Buf.Selection.Start.Line = start
		ctx.Buf.Selection.Start.Char = 0
		ctx.Buf.Selection.End.Line = end
		ctx.Buf.Selection.End.Char = len(CursorLineByNum(ctx.Buf, end).Value) - 1
		CmdEnterInsertMode(ctx)
		yankSave(ctx)
		SelectionDelete(ctx)
		return
	}
	var sel *Selection
	var found bool
	if ch == 'f' {
		found, sel = TextObjectFunction(ctx, include)
	} else if ch == '\'' || ch == '"' || ch == '`' {
		found, sel = TextObjectQuotes(ctx, ch, include)
	} else {
		found, sel = TextObjectBlock(ctx, ch, include)
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

func TextObjectYank(ctx Context, ch rune, include bool) {
	if ctx.Buf == nil {
		return
	}
	if ch == 'w' {
		cur := ContextCursorGet(ctx)
		_, end := TextObjectWord(ctx, false)
		ctx.Buf.Selection = &Selection{
			Start: *cur,
			End:   Cursor{Line: cur.Line, Char: end},
		}
		saveYank(ctx, SelectionToString(ctx.Buf, ctx.Buf.Selection), false, false)
		ctx.Buf.Selection = nil
		return
	}
	if ch == 'W' {
		cur := ContextCursorGet(ctx)
		start, end := TextObjectWord(ctx, true)
		ctx.Buf.Selection = &Selection{
			Start: Cursor{Line: cur.Line, Char: start},
			End:   Cursor{Line: cur.Line, Char: end},
		}
		saveYank(ctx, SelectionToString(ctx.Buf, ctx.Buf.Selection), false, false)
		ctx.Buf.Selection = nil
		return
	}
	if ch == 'p' {
		cur := ContextCursorGet(ctx)
		start, end := getParagraphRange(ctx.Buf, cur.Line, include)
		ctx.Buf.Selection = &Selection{
			Start: Cursor{Line: start, Char: 0},
			End:   Cursor{Line: end, Char: len(CursorLineByNum(ctx.Buf, end).Value) - 1},
		}
		saveYank(ctx, SelectionToString(ctx.Buf, ctx.Buf.Selection), true, false)
		ctx.Buf.Selection = nil
		return
	}
	var sel *Selection
	var found bool
	if ch == 'f' {
		found, sel = TextObjectFunction(ctx, include)
	} else if ch == '\'' || ch == '"' || ch == '`' {
		found, sel = TextObjectQuotes(ctx, ch, include)
	} else {
		found, sel = TextObjectBlock(ctx, ch, include)
	}
	if !found || sel == nil {
		return
	}

	ctx.Buf.Selection = sel
	val := SelectionToString(ctx.Buf, ctx.Buf.Selection)
	saveYank(ctx, val, false, false)
	ctx.Buf.Selection = nil
}

func TextObjectComment(ctx Context, ch rune, include bool) {
	if ctx.Buf == nil {
		return
	}
	if ch == 'p' {
		if include {
			CmdCommentAroundParagraph(ctx)
		} else {
			CmdCommentParagraph(ctx)
		}
		return
	}
	if ch == 'w' || ch == 'W' {
		CmdCommentWord(ctx)
		return
	}
	var sel *Selection
	var found bool
	if ch == 'f' {
		found, sel = TextObjectFunction(ctx, include)
	} else if ch == '\'' || ch == '"' || ch == '`' {
		found, sel = TextObjectQuotes(ctx, ch, include)
	} else {
		found, sel = TextObjectBlock(ctx, ch, include)
	}
	if !found || sel == nil {
		return
	}
	norm := SelectionNormalize(sel)
	ToggleCommentRange(ctx, norm.Start.Line, norm.End.Line)
}

// Concrete commands for text objects
func CmdDeleteInsideFunction(ctx Context) {
	TextObjectDelete(ctx, 'f', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideFunction
}
func CmdDeleteAroundFunction(ctx Context) {
	TextObjectDelete(ctx, 'f', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundFunction
}
func CmdDeleteInsideWord(ctx Context) {
	TextObjectDelete(ctx, 'w', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideWord
}
func CmdDeleteAroundWord(ctx Context) {
	TextObjectDelete(ctx, 'w', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundWord
}
func CmdDeleteInsideParagraph(ctx Context) {
	TextObjectDelete(ctx, 'p', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideParagraph
}
func CmdDeleteAroundParagraph(ctx Context) {
	TextObjectDelete(ctx, 'p', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundParagraph
}
func CmdDeleteInsideQuotesDouble(ctx Context) {
	TextObjectDelete(ctx, '"', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideQuotesDouble
}
func CmdDeleteAroundQuotesDouble(ctx Context) {
	TextObjectDelete(ctx, '"', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundQuotesDouble
}
func CmdDeleteInsideQuotesSingle(ctx Context) {
	TextObjectDelete(ctx, '\'', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideQuotesSingle
}
func CmdDeleteAroundQuotesSingle(ctx Context) {
	TextObjectDelete(ctx, '\'', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundQuotesSingle
}
func CmdDeleteInsideQuotesBacktick(ctx Context) {
	TextObjectDelete(ctx, '`', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideQuotesBacktick
}
func CmdDeleteAroundQuotesBacktick(ctx Context) {
	TextObjectDelete(ctx, '`', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundQuotesBacktick
}
func CmdDeleteInsideParen(ctx Context) {
	TextObjectDelete(ctx, '(', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideParen
}
func CmdDeleteAroundParen(ctx Context) {
	TextObjectDelete(ctx, '(', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundParen
}
func CmdDeleteInsideBrace(ctx Context) {
	TextObjectDelete(ctx, '{', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideBrace
}
func CmdDeleteAroundBrace(ctx Context) {
	TextObjectDelete(ctx, '{', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundBrace
}
func CmdDeleteInsideBracket(ctx Context) {
	TextObjectDelete(ctx, '[', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideBracket
}
func CmdDeleteAroundBracket(ctx Context) {
	TextObjectDelete(ctx, '[', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundBracket
}
func CmdDeleteInsideAngle(ctx Context) {
	TextObjectDelete(ctx, '<', false)
	ctx.Editor.LastRepeatableFn = CmdDeleteInsideAngle
}
func CmdDeleteAroundAngle(ctx Context) {
	TextObjectDelete(ctx, '<', true)
	ctx.Editor.LastRepeatableFn = CmdDeleteAroundAngle
}

func CmdChangeInsideFunction(ctx Context) {
	TextObjectChange(ctx, 'f', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideFunction
}
func CmdChangeAroundFunction(ctx Context) {
	TextObjectChange(ctx, 'f', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundFunction
}
func CmdChangeInsideWord(ctx Context) {
	TextObjectChange(ctx, 'w', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideWord
}
func CmdChangeAroundWord(ctx Context) {
	TextObjectChange(ctx, 'w', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundWord
}
func CmdChangeInsideParagraph(ctx Context) {
	TextObjectChange(ctx, 'p', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideParagraph
}
func CmdChangeAroundParagraph(ctx Context) {
	TextObjectChange(ctx, 'p', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundParagraph
}
func CmdChangeInsideQuotesDouble(ctx Context) {
	TextObjectChange(ctx, '"', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideQuotesDouble
}
func CmdChangeAroundQuotesDouble(ctx Context) {
	TextObjectChange(ctx, '"', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundQuotesDouble
}
func CmdChangeInsideQuotesSingle(ctx Context) {
	TextObjectChange(ctx, '\'', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideQuotesSingle
}
func CmdChangeAroundQuotesSingle(ctx Context) {
	TextObjectChange(ctx, '\'', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundQuotesSingle
}
func CmdChangeInsideQuotesBacktick(ctx Context) {
	TextObjectChange(ctx, '`', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideQuotesBacktick
}
func CmdChangeAroundQuotesBacktick(ctx Context) {
	TextObjectChange(ctx, '`', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundQuotesBacktick
}
func CmdChangeInsideParen(ctx Context) {
	TextObjectChange(ctx, '(', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideParen
}
func CmdChangeAroundParen(ctx Context) {
	TextObjectChange(ctx, '(', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundParen
}
func CmdChangeInsideBrace(ctx Context) {
	TextObjectChange(ctx, '{', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideBrace
}
func CmdChangeAroundBrace(ctx Context) {
	TextObjectChange(ctx, '{', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundBrace
}
func CmdChangeInsideBracket(ctx Context) {
	TextObjectChange(ctx, '[', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideBracket
}
func CmdChangeAroundBracket(ctx Context) {
	TextObjectChange(ctx, '[', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundBracket
}
func CmdChangeInsideAngle(ctx Context) {
	TextObjectChange(ctx, '<', false)
	ctx.Editor.LastRepeatableFn = CmdChangeInsideAngle
}
func CmdChangeAroundAngle(ctx Context) {
	TextObjectChange(ctx, '<', true)
	ctx.Editor.LastRepeatableFn = CmdChangeAroundAngle
}

func CmdYankInsideFunction(ctx Context)       { TextObjectYank(ctx, 'f', false) }
func CmdYankAroundFunction(ctx Context)       { TextObjectYank(ctx, 'f', true) }
func CmdYankInsideWord(ctx Context)           { TextObjectYank(ctx, 'w', false) }
func CmdYankAroundWord(ctx Context)           { TextObjectYank(ctx, 'w', true) }
func CmdYankInsideParagraph(ctx Context)      { TextObjectYank(ctx, 'p', false) }
func CmdYankAroundParagraph(ctx Context)      { TextObjectYank(ctx, 'p', true) }
func CmdYankInsideQuotesDouble(ctx Context)   { TextObjectYank(ctx, '"', false) }
func CmdYankAroundQuotesDouble(ctx Context)   { TextObjectYank(ctx, '"', true) }
func CmdYankInsideQuotesSingle(ctx Context)   { TextObjectYank(ctx, '\'', false) }
func CmdYankAroundQuotesSingle(ctx Context)   { TextObjectYank(ctx, '\'', true) }
func CmdYankInsideQuotesBacktick(ctx Context) { TextObjectYank(ctx, '`', false) }
func CmdYankAroundQuotesBacktick(ctx Context) { TextObjectYank(ctx, '`', true) }
func CmdYankInsideParen(ctx Context)          { TextObjectYank(ctx, '(', false) }
func CmdYankAroundParen(ctx Context)          { TextObjectYank(ctx, '(', true) }
func CmdYankInsideBrace(ctx Context)          { TextObjectYank(ctx, '{', false) }
func CmdYankAroundBrace(ctx Context)          { TextObjectYank(ctx, '{', true) }
func CmdYankInsideBracket(ctx Context)        { TextObjectYank(ctx, '[', false) }
func CmdYankAroundBracket(ctx Context)        { TextObjectYank(ctx, '[', true) }
func CmdYankInsideAngle(ctx Context)          { TextObjectYank(ctx, '<', false) }
func CmdYankAroundAngle(ctx Context)          { TextObjectYank(ctx, '<', true) }

func CmdCommentInsideFunction(ctx Context) {
	TextObjectComment(ctx, 'f', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideFunction
}
func CmdCommentAroundFunction(ctx Context) {
	TextObjectComment(ctx, 'f', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundFunction
}
func CmdCommentInsideWord(ctx Context) {
	TextObjectComment(ctx, 'w', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideWord
}
func CmdCommentAroundWord(ctx Context) {
	TextObjectComment(ctx, 'w', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundWord
}
func CmdCommentInsideParagraph(ctx Context) {
	TextObjectComment(ctx, 'p', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideParagraph
}
func CmdCommentAroundParagraphText(ctx Context) {
	TextObjectComment(ctx, 'p', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundParagraphText
}
func CmdCommentInsideQuotesDouble(ctx Context) {
	TextObjectComment(ctx, '"', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideQuotesDouble
}
func CmdCommentAroundQuotesDouble(ctx Context) {
	TextObjectComment(ctx, '"', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundQuotesDouble
}
func CmdCommentInsideQuotesSingle(ctx Context) {
	TextObjectComment(ctx, '\'', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideQuotesSingle
}
func CmdCommentAroundQuotesSingle(ctx Context) {
	TextObjectComment(ctx, '\'', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundQuotesSingle
}
func CmdCommentInsideQuotesBacktick(ctx Context) {
	TextObjectComment(ctx, '`', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideQuotesBacktick
}
func CmdCommentAroundQuotesBacktick(ctx Context) {
	TextObjectComment(ctx, '`', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundQuotesBacktick
}
func CmdCommentInsideParen(ctx Context) {
	TextObjectComment(ctx, '(', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideParen
}
func CmdCommentAroundParen(ctx Context) {
	TextObjectComment(ctx, '(', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundParen
}
func CmdCommentInsideBrace(ctx Context) {
	TextObjectComment(ctx, '{', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideBrace
}
func CmdCommentAroundBrace(ctx Context) {
	TextObjectComment(ctx, '{', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundBrace
}
func CmdCommentInsideBracket(ctx Context) {
	TextObjectComment(ctx, '[', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideBracket
}
func CmdCommentAroundBracket(ctx Context) {
	TextObjectComment(ctx, '[', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundBracket
}
func CmdCommentInsideAngle(ctx Context) {
	TextObjectComment(ctx, '<', false)
	ctx.Editor.LastRepeatableFn = CmdCommentInsideAngle
}
func CmdCommentAroundAngle(ctx Context) {
	TextObjectComment(ctx, '<', true)
	ctx.Editor.LastRepeatableFn = CmdCommentAroundAngle
}

func MakeTextObjectKeyMap(inside bool, op string) KeyMap {
	switch op {
	case "delete":
		if inside {
			return KeyMap{
				"f":  CmdDeleteInsideFunction,
				"w":  CmdDeleteInsideWord,
				"p":  CmdDeleteInsideParagraph,
				"\"": CmdDeleteInsideQuotesDouble,
				"'":  CmdDeleteInsideQuotesSingle,
				"`":  CmdDeleteInsideQuotesBacktick,
				"(":  CmdDeleteInsideParen,
				")":  CmdDeleteInsideParen,
				"b":  CmdDeleteInsideParen,
				"{":  CmdDeleteInsideBrace,
				"}":  CmdDeleteInsideBrace,
				"B":  CmdDeleteInsideBrace,
				"[":  CmdDeleteInsideBracket,
				"]":  CmdDeleteInsideBracket,
				"<":  CmdDeleteInsideAngle,
				">":  CmdDeleteInsideAngle,
			}
		}
		return KeyMap{
			"f":  CmdDeleteAroundFunction,
			"w":  CmdDeleteAroundWord,
			"p":  CmdDeleteAroundParagraph,
			"\"": CmdDeleteAroundQuotesDouble,
			"'":  CmdDeleteAroundQuotesSingle,
			"`":  CmdDeleteAroundQuotesBacktick,
			"(":  CmdDeleteAroundParen,
			")":  CmdDeleteAroundParen,
			"b":  CmdDeleteAroundParen,
			"{":  CmdDeleteAroundBrace,
			"}":  CmdDeleteAroundBrace,
			"B":  CmdDeleteAroundBrace,
			"[":  CmdDeleteAroundBracket,
			"]":  CmdDeleteAroundBracket,
			"<":  CmdDeleteAroundAngle,
			">":  CmdDeleteAroundAngle,
		}
	case "change":
		if inside {
			return KeyMap{
				"f":  CmdChangeInsideFunction,
				"w":  CmdChangeInsideWord,
				"W":  CmdChangeWORD,
				"p":  CmdChangeInsideParagraph,
				"\"": CmdChangeInsideQuotesDouble,
				"'":  CmdChangeInsideQuotesSingle,
				"`":  CmdChangeInsideQuotesBacktick,
				"(":  CmdChangeInsideParen,
				")":  CmdChangeInsideParen,
				"b":  CmdChangeInsideParen,
				"{":  CmdChangeInsideBrace,
				"}":  CmdChangeInsideBrace,
				"B":  CmdChangeInsideBrace,
				"[":  CmdChangeInsideBracket,
				"]":  CmdChangeInsideBracket,
				"<":  CmdChangeInsideAngle,
				">":  CmdChangeInsideAngle,
			}
		}
		return KeyMap{
			"f":  CmdChangeAroundFunction,
			"w":  CmdChangeAroundWord,
			"p":  CmdChangeAroundParagraph,
			"\"": CmdChangeAroundQuotesDouble,
			"'":  CmdChangeAroundQuotesSingle,
			"`":  CmdChangeAroundQuotesBacktick,
			"(":  CmdChangeAroundParen,
			")":  CmdChangeAroundParen,
			"b":  CmdChangeAroundParen,
			"{":  CmdChangeAroundBrace,
			"}":  CmdChangeAroundBrace,
			"B":  CmdChangeAroundBrace,
			"[":  CmdChangeAroundBracket,
			"]":  CmdChangeAroundBracket,
			"<":  CmdChangeAroundAngle,
			">":  CmdChangeAroundAngle,
		}
	case "yank":
		if inside {
			return KeyMap{
				"f":  CmdYankInsideFunction,
				"w":  CmdYankInsideWord,
				"p":  CmdYankInsideParagraph,
				"\"": CmdYankInsideQuotesDouble,
				"'":  CmdYankInsideQuotesSingle,
				"`":  CmdYankInsideQuotesBacktick,
				"(":  CmdYankInsideParen,
				")":  CmdYankInsideParen,
				"b":  CmdYankInsideParen,
				"{":  CmdYankInsideBrace,
				"}":  CmdYankInsideBrace,
				"B":  CmdYankInsideBrace,
				"[":  CmdYankInsideBracket,
				"]":  CmdYankInsideBracket,
				"<":  CmdYankInsideAngle,
				">":  CmdYankInsideAngle,
			}
		}
		return KeyMap{
			"f":  CmdYankAroundFunction,
			"w":  CmdYankAroundWord,
			"p":  CmdYankAroundParagraph,
			"\"": CmdYankAroundQuotesDouble,
			"'":  CmdYankAroundQuotesSingle,
			"`":  CmdYankAroundQuotesBacktick,
			"(":  CmdYankAroundParen,
			")":  CmdYankAroundParen,
			"b":  CmdYankAroundParen,
			"{":  CmdYankAroundBrace,
			"}":  CmdYankAroundBrace,
			"B":  CmdYankAroundBrace,
			"[":  CmdYankAroundBracket,
			"]":  CmdYankAroundBracket,
			"<":  CmdYankAroundAngle,
			">":  CmdYankAroundAngle,
		}
	case "comment":
		if inside {
			return KeyMap{
				"f":  CmdCommentInsideFunction,
				"w":  CmdCommentInsideWord,
				"p":  CmdCommentInsideParagraph,
				"\"": CmdCommentInsideQuotesDouble,
				"'":  CmdCommentInsideQuotesSingle,
				"`":  CmdCommentInsideQuotesBacktick,
				"(":  CmdCommentInsideParen,
				")":  CmdCommentInsideParen,
				"b":  CmdCommentInsideParen,
				"{":  CmdCommentInsideBrace,
				"}":  CmdCommentInsideBrace,
				"B":  CmdCommentInsideBrace,
				"[":  CmdCommentInsideBracket,
				"]":  CmdCommentInsideBracket,
				"<":  CmdCommentInsideAngle,
				">":  CmdCommentInsideAngle,
			}
		}
		return KeyMap{
			"f":  CmdCommentAroundFunction,
			"w":  CmdCommentAroundWord,
			"p":  CmdCommentAroundParagraphText,
			"\"": CmdCommentAroundQuotesDouble,
			"'":  CmdCommentAroundQuotesSingle,
			"`":  CmdCommentAroundQuotesBacktick,
			"(":  CmdCommentAroundParen,
			")":  CmdCommentAroundParen,
			"b":  CmdCommentAroundParen,
			"{":  CmdCommentAroundBrace,
			"}":  CmdCommentAroundBrace,
			"B":  CmdCommentAroundBrace,
			"[":  CmdCommentAroundBracket,
			"]":  CmdCommentAroundBracket,
			"<":  CmdCommentAroundAngle,
			">":  CmdCommentAroundAngle,
		}
	}
	return KeyMap{}
}

func CmdChangeInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectChange(ctx, rune(ctx.Char[0]), false)
	}
}

func CmdChangeAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectChange(ctx, rune(ctx.Char[0]), true)
	}
}

func CmdDeleteInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectDelete(ctx, rune(ctx.Char[0]), false)
	}
}

func CmdDeleteAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectDelete(ctx, rune(ctx.Char[0]), true)
	}
}

func CmdYankInside(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectYank(ctx, rune(ctx.Char[0]), false)
	}
}

func CmdYankAround(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		TextObjectYank(ctx, rune(ctx.Char[0]), true)
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

// CmdSetMark waits for a character input and sets a mark at the current cursor position globally.
func CmdSetMark(ctx Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		r := rune(ctx.Char[0])
		if ctx.Editor.Marks == nil {
			ctx.Editor.Marks = make(map[rune]Mark)
		}
		cur := ContextCursorGet(ctx)
		ctx.Editor.Marks[r] = Mark{
			Buf:    ctx.Buf,
			Cursor: *cur,
		}
		ctx.Editor.EchoMessage("Mark '" + string(r) + "' set")
	}
}

// CmdGotoMark opens the global marks popup.
func CmdGotoMark(ctx Context) {
	if MarksPopupFactory != nil {
		MarksPopupFactory(ctx, ctx.Editor.Marks)
	}
}

// CmdDummyNA is a no-op command used to disable or override keybindings in config.toml
// by mapping them to this function (e.g. `x = "CmdDummyNA"`).
func CmdDummyNA(ctx Context) {
	// No operation
}
