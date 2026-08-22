package wig

import (
	"strings"

	"github.com/atotto/clipboard"
)

type yank struct {
	val     string
	isLine  bool
	isBlock bool
}

type Register struct {
	Val     string
	IsLine  bool
	IsBlock bool
}

var NamedRegisters = map[rune]Register{}
var LastCommand string

func SetRegister(reg rune, val string, isLine bool, isBlock bool) {
	NamedRegisters[reg] = Register{
		Val:     val,
		IsLine:  isLine,
		IsBlock: isBlock,
	}
}

func GetAlternateBufferName(ctx Context) string {
	if ctx.Editor == nil || ctx.Editor.ActiveWindow() == nil {
		return ""
	}
	win := ctx.Editor.ActiveWindow()
	curBuf := win.Buffer()
	if curBuf == nil {
		return ""
	}
	for item := win.Jumps.List.Last(); item != nil; item = item.Prev() {
		if item.Value.FilePath != curBuf.FilePath && item.Value.FilePath != "" {
			return item.Value.FilePath
		}
	}
	return ""
}

func GetRegisterText(ctx Context, reg rune) string {
	switch reg {
	case '%':
		if ctx.Buf != nil && ctx.Buf.GetName() != "" {
			return ctx.Buf.GetName()
		}
		return ""
	case '#':
		return GetAlternateBufferName(ctx)
	case ':':
		return LastCommand
	case '/':
		return LastSearchPattern
	case '+', '*':
		text, err := clipboard.ReadAll()
		if err == nil {
			return text
		}
		return ""
	case '"':
		if r, ok := NamedRegisters['"']; ok && r.Val != "" {
			return r.Val
		}
		if ctx.Editor != nil && ctx.Editor.Yanks.Len > 0 {
			return ctx.Editor.Yanks.Last().Value.val
		}
		return ""
	case '0':
		if r, ok := NamedRegisters['0']; ok {
			return r.Val
		}
		return ""
	case '-':
		if r, ok := NamedRegisters['-']; ok {
			return r.Val
		}
		return ""
	case '_':
		return ""
	case '.':
		return LastInsertedText
	default:
		if r, ok := NamedRegisters[reg]; ok {
			return r.Val
		}
	}
	return ""
}

func getActiveRegister(ctx Context) rune {
	if ctx.Editor != nil && ctx.Editor.ActiveRegister != 0 {
		r := ctx.Editor.ActiveRegister
		ctx.Editor.ActiveRegister = 0
		return r
	}
	return '"'
}

func saveYank(ctx Context, text string, isLine bool, isBlock bool) {
	if text == "" {
		return
	}
	reg := getActiveRegister(ctx)
	if reg == '_' {
		return // Black hole register: discard
	}

	// 1. Always update dedicated yank register '0'
	SetRegister('0', text, isLine, isBlock)

	// 2. Always update default unnamed register '"'
	SetRegister('"', text, isLine, isBlock)

	// 3. Update target register if specified
	switch {
	case reg == '+' || reg == '*':
		_ = clipboard.WriteAll(text)
	case reg >= 'a' && reg <= 'z':
		SetRegister(reg, text, isLine, isBlock)
	case reg >= 'A' && reg <= 'Z':
		lower := reg + ('a' - 'A')
		existing := NamedRegisters[lower].Val
		SetRegister(lower, existing+text, isLine, isBlock)
	}
}

func saveDelete(ctx Context, text string, isLine bool, isBlock bool) {
	if text == "" {
		return
	}
	reg := getActiveRegister(ctx)
	if reg == '_' {
		return // Black hole register: discard
	}

	// 1. If small deletion (< 1 line, no newline), store in '-'. Otherwise shift '1'-'9'.
	if !isLine && !strings.Contains(text, "\n") {
		SetRegister('-', text, isLine, isBlock)
	} else {
		for i := 9; i > 1; i-- {
			prevKey := rune('0' + i - 1)
			currKey := rune('0' + i)
			if r, ok := NamedRegisters[prevKey]; ok {
				NamedRegisters[currKey] = r
			}
		}
		SetRegister('1', text, isLine, isBlock)
	}

	// 2. Always update default unnamed register '"'
	SetRegister('"', text, isLine, isBlock)

	// 3. Update target register if specified
	switch {
	case reg == '+' || reg == '*':
		_ = clipboard.WriteAll(text)
	case reg >= 'a' && reg <= 'z':
		SetRegister(reg, text, isLine, isBlock)
	case reg >= 'A' && reg <= 'Z':
		lower := reg + ('a' - 'A')
		existing := NamedRegisters[lower].Val
		SetRegister(lower, existing+text, isLine, isBlock)
	}
}

func getPutText(ctx Context) (text string, isLine bool, isBlock bool) {
	reg := getActiveRegister(ctx)
	if reg == '+' || reg == '*' {
		clip, err := clipboard.ReadAll()
		if err == nil {
			return clip, false, false
		}
		return "", false, false
	}
	if reg == '%' {
		if ctx.Buf != nil {
			return ctx.Buf.GetName(), false, false
		}
		return "", false, false
	}
	if reg != '"' && reg != 0 {
		if r, ok := NamedRegisters[reg]; ok {
			return r.Val, r.IsLine, r.IsBlock
		}
		return "", false, false
	}

	// Default unnamed register
	if ctx.Editor != nil && ctx.Editor.Yanks.Len > 0 {
		last := ctx.Editor.Yanks.Last().Value
		return last.val, last.isLine, last.isBlock
	}
	if r, ok := NamedRegisters['"']; ok {
		return r.Val, r.IsLine, r.IsBlock
	}
	return "", false, false
}

func CmdSelectRegister(ctx Context) {
	if RegistersPopupFactory != nil {
		RegistersPopupFactory(ctx)
	}
}

// CmdInsertRegister handles <Ctrl-r>{reg} insertion in Insert mode.
func CmdInsertRegister(_ Context) func(Context) {
	return func(ctx Context) {
		if len(ctx.Char) == 0 {
			return
		}
		r := rune(ctx.Char[0])
		text := GetRegisterText(ctx, r)
		if text != "" {
			cur := ContextCursorGet(ctx)
			line := CursorLine(ctx.Buf, cur)
			TextInsert(ctx.Buf, line, cur.Char, text)
			for range text {
				CursorInc(ctx.Buf, cur)
			}
		}
	}
}

func GetYankHistory(maxCount int) []string {
	if EditorInst == nil {
		return nil
	}
	var res []string
	idx := 0
	for elem := EditorInst.Yanks.Last(); elem != nil && idx < maxCount; elem = elem.Prev() {
		if elem.Value.val != "" {
			res = append(res, elem.Value.val)
			idx++
		}
	}
	return res
}

func CmdShowRegisters(ctx Context) {
	if RegistersPopupFactory != nil {
		RegistersPopupFactory(ctx)
	}
}

type Yanks struct {
	items List[yank]
}

func CmdYank(ctx Context) {
	cur := ContextCursorGet(ctx)
	defer CmdNormalMode(ctx)
	defer func() {
		if ctx.Buf.Selection != nil {
			cur.Line = ctx.Buf.Selection.Start.Line
			cur.Char = ctx.Buf.Selection.Start.Char
		}
		ctx.Buf.Selection = nil
	}()

	// Visual mode: yank selection as-is
	if ctx.Buf.Selection != nil {
		yankSave(ctx)
		return
	}

	// Normal mode: yank N lines (linewise)
	count := max(int(ctx.Count), 1)
	endLine := min(cur.Line+count-1, ctx.Buf.Lines.Len-1)

	var text strings.Builder
	for i := cur.Line; i <= endLine; i++ {
		line := CursorLineByNum(ctx.Buf, i)
		if line != nil {
			text.WriteString(string(line.Value))
		}
	}

	y := yank{val: text.String(), isLine: true}
	if ctx.Editor.Yanks.Len == 0 || ctx.Editor.Yanks.Last().Value != y {
		ctx.Editor.Yanks.PushBack(y)
	}
	saveYank(ctx, y.val, y.isLine, y.isBlock)

	repeatCount := ctx.Count
	ctx.Editor.LastRepeatableFn = func(c Context) {
		if c.Count == 0 {
			c.Count = repeatCount
		}
		CmdYank(c)
	}
}

func CmdYankEol(ctx Context) {
	cur := ContextCursorGet(ctx)
	defer CmdNormalMode(ctx)
	defer func() {
		if ctx.Buf.Selection != nil {
			cur.Line = ctx.Buf.Selection.Start.Line
			cur.Char = ctx.Buf.Selection.Start.Char
		}
		ctx.Buf.Selection = nil
	}()
	SelectionStart(ctx.Buf, cur)
	WithSelection(CmdGotoLineEnd)(ctx)
	CmdCursorLeft(ctx)
	SelectionStop(ctx.Buf, cur)
	yankSave(ctx)
}

func CmdYankBeforeChar(ctx Context) func(Context) {
	return func(ctx Context) {
		startCur := *ContextCursorGet(ctx)
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardBeforeChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		yankSave(ctx)
		ctx.Buf.Selection = nil
		*cur = startCur
	}
}

func CmdYankToChar(_ Context) func(Context) {
	return func(ctx Context) {
		startCur := *ContextCursorGet(ctx)
		cur := ContextCursorGet(ctx)
		SelectionStart(ctx.Buf, cur)
		CmdForwardToChar(ctx)(ctx)
		SelectionStop(ctx.Buf, cur)
		yankSave(ctx)
		ctx.Buf.Selection = nil
		*cur = startCur
	}
}

func CmdYankPut(ctx Context) {
	cur := ContextCursorGet(ctx)
	val, isLine, isBlock := getPutText(ctx)
	if val == "" {
		return
	}
	if isBlock {
		blockPut(ctx, false, val)
		return
	}
	if ctx.Buf.Selection != nil {
		if ctx.Buf.TxStart() {
			defer yankSave(ctx, SelectionToString(ctx.Buf, ctx.Buf.Selection))
			if ctx.Buf.Mode() == MODE_VISUAL {
				SelectionDelete(ctx)
			}
			if ctx.Buf.Mode() == MODE_VISUAL_LINE {
				SelectionDelete(ctx)
				CmdCursorLineUp(ctx)
				line := CursorLine(ctx.Buf, cur)
				CmdAppendLine(ctx)
				TextInsert(ctx.Buf, line, len(line.Value)-1, "\n")
			}
			ctx.Buf.TxEnd()
		}
	}

	if isLine {
		CmdCursorLineDown(ctx)
		CmdYankPutBefore(ctx)
		return
	}

	CmdEnterInsertMode(ctx)
	defer CmdExitInsertMode(ctx)

	CmdCursorRight(ctx)
	yankPutText(ctx, val)
}

func CmdYankPutBefore(ctx Context) {
	val, isLine, isBlock := getPutText(ctx)
	if val == "" {
		return
	}
	if isBlock {
		blockPut(ctx, true, val)
		return
	}
	cur := ContextCursorGet(ctx)
	CmdEnterInsertMode(ctx)
	defer CmdExitInsertMode(ctx)

	if isLine {
		CmdLineOpenAbove(ctx)
		CmdCursorBeginningOfTheLine(ctx)

		// clear any indentation
		SelectionStart(ctx.Buf, cur)
		CmdGotoLineEnd(ctx)
		SelectionStop(ctx.Buf, cur)
		SelectionDelete(ctx)

		yankPutText(ctx, val)
	} else {
		yankPutText(ctx, val)
	}
}

func yankSave(ctx Context, text ...string) {
	cur := ContextCursorGet(ctx)
	var y yank
	line := CursorLine(ctx.Buf, cur)

	if len(text) == 0 {
		if ctx.Buf.Selection == nil {
			y = yank{val: string(line.Value)}
		} else {
			st := SelectionToString(ctx.Buf, ctx.Buf.Selection)
			if len(st) == 0 {
				return
			}
			y = yank{val: st}
		}
	} else {
		y = yank{val: text[0]}
	}

	y.isLine = (ctx.Buf.Mode() == MODE_VISUAL_LINE) || ctx.Buf.Selection == nil

	if ctx.Editor.Yanks.Len == 0 {
		ctx.Editor.Yanks.PushBack(y)
	} else if ctx.Editor.Yanks.Last().Value != y {
		ctx.Editor.Yanks.PushBack(y)
	}

	saveDelete(ctx, y.val, y.isLine, y.isBlock)
}

func yankPut(ctx Context) {
	v := ctx.Editor.Yanks.Last()
	if v == nil {
		return
	}
	yankPutText(ctx, v.Value.val)
}

func yankPutText(ctx Context, text string) {
	cur := ContextCursorGet(ctx)
	TextInsert(ctx.Buf, CursorLine(ctx.Buf, cur), cur.Char, text)
	i := len(text)
	for i >= 1 {
		i--
		CursorInc(ctx.Buf, cur)
	}
}

// blockPut inserts a blockwise register at a fixed column across successive
// lines (one register line per buffer line), padding short lines with spaces
// and appending new lines at EOF if the block extends past the last line —
// unlike yankPut's single flat stream insert.
func blockPut(ctx Context, before bool, textVal ...string) {
	cur := ContextCursorGet(ctx)
	val := ""
	if len(textVal) > 0 && textVal[0] != "" {
		val = textVal[0]
	} else if ctx.Editor.Yanks.Len > 0 {
		val = ctx.Editor.Yanks.Last().Value.val
	} else {
		return
	}
	lines := strings.Split(val, "\n")
	startChar := cur.Char
	if !before {
		startChar++
	}
	startLine := cur.Line
	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}
	for i, text := range lines {
		lineNum := startLine + i
		line := CursorLineByNum(ctx.Buf, lineNum)
		if line == nil {
			last := ctx.Buf.Lines.Last()
			TextInsert(ctx.Buf, last, len(last.Value)-1, "\n"+strings.Repeat(" ", startChar)+text)
			continue
		}
		lineLen := len(line.Value) - 1 // exclude trailing "\n"
		if startChar > lineLen {
			TextInsert(ctx.Buf, line, lineLen, strings.Repeat(" ", startChar-lineLen))
			line = CursorLineByNum(ctx.Buf, lineNum)
		}
		TextInsert(ctx.Buf, line, startChar, text)
	}
	cur.Line = startLine
	cur.Char = startChar
}
