package wig

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

type CursorInstance struct {
	Cursor    Cursor
	Selection *Selection
}

type MultiCursor struct {
	buf          *Buffer
	Cursors      []CursorInstance
	Pattern      string
	PrimaryIndex int
}

func NewMultiCursor(buf *Buffer) *MultiCursor {
	return &MultiCursor{
		buf:          buf,
		Cursors:      make([]CursorInstance, 0),
		PrimaryIndex: 0,
	}
}

func (m *MultiCursor) Active() bool {
	return len(m.Cursors) > 1 || (len(m.Cursors) == 1 && m.Pattern != "")
}

func (m *MultiCursor) Count() int {
	return len(m.Cursors)
}

func (m *MultiCursor) Clear() {
	m.Cursors = m.Cursors[:0]
	m.Pattern = ""
	m.PrimaryIndex = 0
	if m.buf != nil {
		m.buf.Selection = nil
	}
}

func (m *MultiCursor) HasCursorAt(line, char int) bool {
	for _, c := range m.Cursors {
		if c.Cursor.Line == line && c.Cursor.Char == char {
			return true
		}
	}
	return false
}

func (m *MultiCursor) HasSelectionAt(line, char int) bool {
	for _, c := range m.Cursors {
		if c.Selection != nil && SelectionCursorInRange(c.Selection, Cursor{Line: line, Char: char}) {
			return true
		}
	}
	return false
}

func (m *MultiCursor) Sort() {
	if len(m.Cursors) <= 1 {
		return
	}
	var primCur Cursor
	hasPrim := m.PrimaryIndex >= 0 && m.PrimaryIndex < len(m.Cursors)
	if hasPrim {
		primCur = m.Cursors[m.PrimaryIndex].Cursor
	}
	sort.Slice(m.Cursors, func(i, j int) bool {
		if m.Cursors[i].Cursor.Line != m.Cursors[j].Cursor.Line {
			return m.Cursors[i].Cursor.Line < m.Cursors[j].Cursor.Line
		}
		return m.Cursors[i].Cursor.Char < m.Cursors[j].Cursor.Char
	})
	if hasPrim {
		for i, c := range m.Cursors {
			if c.Cursor.Line == primCur.Line && c.Cursor.Char == primCur.Char {
				m.PrimaryIndex = i
				break
			}
		}
	}
}

func (m *MultiCursor) MoveLeft(count uint32) {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		for i := uint32(0); i < count; i++ {
			if cur.Char > 0 {
				cur.Char--
				cur.PreserveCharPosition = cur.Char
			}
		}
		m.Cursors[idx].Selection = nil
	}
}

func (m *MultiCursor) MoveRight(count uint32) {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		line := CursorLine(m.buf, cur)
		if line == nil {
			continue
		}
		for i := uint32(0); i < count; i++ {
			if cur.Char < len(line.Value)-1 {
				cur.Char++
				cur.PreserveCharPosition = cur.Char
			}
		}
		m.Cursors[idx].Selection = nil
	}
}

// CollapseToInsert turns each cursor's selection into a plain insertion
// point — at the selection start (atEnd=false, "i") or just past its end
// (atEnd=true, "a") — without deleting any text.
func (m *MultiCursor) CollapseToInsert(atEnd bool) {
	for idx := range m.Cursors {
		ci := &m.Cursors[idx]
		cur := &ci.Cursor
		if ci.Selection != nil {
			sel := SelectionNormalize(ci.Selection)
			if atEnd {
				cur.Line = sel.End.Line
				cur.Char = sel.End.Char + 1
			} else {
				cur.Line = sel.Start.Line
				cur.Char = sel.Start.Char
			}
			ci.Selection = nil
		} else if atEnd {
			line := CursorLine(m.buf, cur)
			if line != nil && cur.Char < len(line.Value)-1 {
				cur.Char++
			}
		}
		cur.PreserveCharPosition = cur.Char
	}
	m.buf.Selection = nil
}

func (m *MultiCursor) MoveUp(count uint32) {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		cur.Line = max(cur.Line-int(count), 0)
		restoreCharPosition(m.buf, cur)
		m.Cursors[idx].Selection = nil
	}
}

func (m *MultiCursor) MoveDown(count uint32) {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		cur.Line = min(cur.Line+int(count), m.buf.Lines.Len-1)
		restoreCharPosition(m.buf, cur)
		m.Cursors[idx].Selection = nil
	}
}

func (m *MultiCursor) MoveHome() {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		cur.Char = 0
		cur.PreserveCharPosition = 0
		m.Cursors[idx].Selection = nil
	}
}

func (m *MultiCursor) MoveEnd() {
	for idx := range m.Cursors {
		cur := &m.Cursors[idx].Cursor
		line := CursorLine(m.buf, cur)
		if line == nil {
			continue
		}
		cur.Char = len(line.Value) - 1
		cur.PreserveCharPosition = cur.Char
		m.Cursors[idx].Selection = nil
	}
}

func (m *MultiCursor) MatchNextOccurrence(ctx Context) {
	cur := ContextCursorGet(ctx)
	if cur == nil || ctx.Buf == nil {
		return
	}

	if len(m.Cursors) == 0 || m.Pattern == "" {
		if ctx.Buf.Selection != nil {
			sel := SelectionNormalize(ctx.Buf.Selection)
			text := SelectionToString(ctx.Buf, &sel)
			if text == "" {
				return
			}
			m.Pattern = text
			curCopy := *cur
			m.Cursors = append(m.Cursors, CursorInstance{
				Cursor:    curCopy,
				Selection: &sel,
			})
			m.PrimaryIndex = 0
		} else {
			line := CursorLine(ctx.Buf, cur)
			if line == nil || len(line.Value) == 0 {
				return
			}
			start, end := TextObjectWord(ctx, true)
			if start > end || start >= len(line.Value) {
				return
			}
			word := string(line.Value.Range(start, end+1))
			word = strings.TrimSpace(word)
			if word == "" {
				return
			}
			m.Pattern = word
			sel := Selection{
				Start: Cursor{Line: cur.Line, Char: start},
				End:   Cursor{Line: cur.Line, Char: end},
			}
			curCopy := *cur
			curCopy.Char = end
			curCopy.PreserveCharPosition = end
			m.Cursors = append(m.Cursors, CursorInstance{
				Cursor:    curCopy,
				Selection: &sel,
			})
			ctx.Buf.Selection = &sel
			*cur = curCopy
			m.PrimaryIndex = 0
			setBufferMode(ctx, MODE_VISUAL)
		}
	}

	last := m.Cursors[len(m.Cursors)-1]
	fromLine := last.Cursor.Line
	fromChar := last.Cursor.Char + 1
	if last.Selection != nil {
		fromLine = last.Selection.End.Line
		fromChar = last.Selection.End.Char + 1
	}
	m.searchAndAdd(ctx, fromLine, fromChar)
}

// SkipNext removes the most recently added cursor/selection and adds the
// next occurrence after it, effectively skipping the current match.
func (m *MultiCursor) SkipNext(ctx Context) {
	if len(m.Cursors) == 0 || m.Pattern == "" {
		return
	}
	skipped := m.Cursors[len(m.Cursors)-1]
	m.Cursors = m.Cursors[:len(m.Cursors)-1]
	if len(m.Cursors) == 0 {
		m.Clear()
		ctx.Buf.Selection = nil
		setBufferMode(ctx, MODE_NORMAL)
		return
	}
	fromLine := skipped.Cursor.Line
	fromChar := skipped.Cursor.Char + 1
	if skipped.Selection != nil {
		fromLine = skipped.Selection.End.Line
		fromChar = skipped.Selection.End.Char + 1
	}
	prevCount := len(m.Cursors)
	m.searchAndAdd(ctx, fromLine, fromChar)
	if len(m.Cursors) == prevCount && prevCount > 0 {
		if m.PrimaryIndex >= len(m.Cursors) {
			m.PrimaryIndex = len(m.Cursors) - 1
		}
		last := m.Cursors[m.PrimaryIndex]
		ctx.Buf.Selection = last.Selection
		cur := ContextCursorGet(ctx)
		*cur = last.Cursor
	}
}

// searchAndAdd finds the pattern's next occurrence starting from fromLine/fromChar
// (wrapping around the buffer) and appends it as a new cursor/selection.
func (m *MultiCursor) searchAndAdd(ctx Context, fromLine, fromChar int) {
	cur := ContextCursorGet(ctx)
	patRunes := []rune(m.Pattern)
	if len(patRunes) == 0 {
		return
	}
	totalLines := ctx.Buf.Lines.Len
	foundLine := -1
	foundChar := -1
	for l := 0; l < totalLines; l++ {
		lineIdx := (fromLine + l) % totalLines
		lineElem := CursorLineByNum(ctx.Buf, lineIdx)
		if lineElem == nil {
			continue
		}
		startC := 0
		if l == 0 {
			startC = fromChar
		}
		if startC >= len(lineElem.Value) {
			continue
		}
		idx := indexOfRunes(lineElem.Value[startC:], patRunes)
		if idx >= 0 {
			candidateLine := lineIdx
			candidateChar := startC + idx
			alreadyExists := false
			for _, c := range m.Cursors {
				if c.Selection != nil && c.Selection.Start.Line == candidateLine && c.Selection.Start.Char == candidateChar {
					alreadyExists = true
					break
				}
			}
			if alreadyExists {
				ctx.Editor.EchoMessage("no more matches")
				return
			}
			foundLine = candidateLine
			foundChar = candidateChar
			break
		}
	}
	if foundLine == -1 {
		ctx.Editor.EchoMessage("no more matches")
		return
	}
	newSel := Selection{
		Start: Cursor{Line: foundLine, Char: foundChar},
		End:   Cursor{Line: foundLine, Char: foundChar + len(patRunes) - 1},
	}
	newCur := Cursor{
		Line:                 foundLine,
		Char:                 foundChar + len(patRunes) - 1,
		PreserveCharPosition: foundChar + len(patRunes) - 1,
	}
	m.Cursors = append(m.Cursors, CursorInstance{
		Cursor:    newCur,
		Selection: &newSel,
	})
	m.PrimaryIndex = len(m.Cursors) - 1
	ctx.Buf.Selection = &newSel
	*cur = newCur
	setBufferMode(ctx, MODE_VISUAL)
	CmdEnsureCursorVisible(ctx)
	m.Cursors[m.PrimaryIndex].Cursor.ScrollOffset = cur.ScrollOffset
	ctx.Editor.EchoMessage(fmt.Sprintf("%d selections", len(m.Cursors)))
}

func (m *MultiCursor) DeleteSelections(ctx Context) {
	if len(m.Cursors) == 0 {
		return
	}
	m.Sort()

	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}

	for i := len(m.Cursors) - 1; i >= 0; i-- {
		ci := &m.Cursors[i]
		var sel Selection
		var deletedLen int

		if ci.Selection != nil {
			norm := SelectionNormalize(ci.Selection)
			norm.End.Char++
			deletedLen = norm.End.Char - norm.Start.Char
			sel = norm
		} else {
			line := CursorLineByNum(ctx.Buf, ci.Cursor.Line)
			if line == nil || len(line.Value) <= 1 {
				continue
			}
			count := max(int(ctx.Count), 1)
			delChar := ci.Cursor.Char
			if delChar >= len(line.Value)-1 {
				delChar = max(0, len(line.Value)-2)
			}
			endChar := min(delChar+count, len(line.Value)-1)
			deletedLen = endChar - delChar
			if deletedLen <= 0 {
				continue
			}
			sel = Selection{
				Start: Cursor{Line: ci.Cursor.Line, Char: delChar},
				End:   Cursor{Line: ci.Cursor.Line, Char: endChar},
			}
		}

		TextDelete(ctx.Buf, &sel)
		ci.Cursor = sel.Start
		ci.Cursor.PreserveCharPosition = sel.Start.Char
		ci.Selection = nil

		for k := i + 1; k < len(m.Cursors); k++ {
			if m.Cursors[k].Cursor.Line == sel.Start.Line {
				m.Cursors[k].Cursor.Char = max(0, m.Cursors[k].Cursor.Char-deletedLen)
				m.Cursors[k].Cursor.PreserveCharPosition = m.Cursors[k].Cursor.Char
			}
		}
	}

	ctx.Buf.Selection = nil
	if len(m.Cursors) > 0 {
		if m.PrimaryIndex >= len(m.Cursors) || m.PrimaryIndex < 0 {
			m.PrimaryIndex = len(m.Cursors) - 1
		}
		cur := ContextCursorGet(ctx)
		savedOffset := cur.ScrollOffset
		*cur = m.Cursors[m.PrimaryIndex].Cursor
		cur.ScrollOffset = savedOffset
		CmdEnsureCursorVisible(ctx)
	}
}

func (m *MultiCursor) HandleInsertKey(ctx Context, ev *tcell.EventKey) bool {
	if ctx.Buf.Mode() != MODE_INSERT {
		return false
	}
	ch := ev.Rune()
	if ev.Key() == tcell.KeyCtrlJ || ev.Key() == tcell.KeyEnter {
		ch = '\n'
	} else if ev.Modifiers()&tcell.ModCtrl != 0 || ev.Modifiers()&tcell.ModAlt != 0 || ev.Modifiers()&tcell.ModMeta != 0 {
		return false
	}

	if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
		m.Sort()
		for i := len(m.Cursors) - 1; i >= 0; i-- {
			cur := &m.Cursors[i].Cursor
			if cur.Char <= 0 {
				continue
			}
			start := *cur
			start.Char--
			TextDelete(m.buf, &Selection{
				Start: start,
				End:   *cur,
			})
			cur.Char--
			cur.PreserveCharPosition = cur.Char

			for k := i + 1; k < len(m.Cursors); k++ {
				if m.Cursors[k].Cursor.Line == cur.Line && m.Cursors[k].Cursor.Char > 0 {
					m.Cursors[k].Cursor.Char--
					m.Cursors[k].Cursor.PreserveCharPosition = m.Cursors[k].Cursor.Char
				}
			}
		}

		if len(m.Cursors) > 0 {
			if m.PrimaryIndex >= len(m.Cursors) || m.PrimaryIndex < 0 {
				m.PrimaryIndex = len(m.Cursors) - 1
			}
			winCur := ContextCursorGet(ctx)
			savedOffset := winCur.ScrollOffset
			*winCur = m.Cursors[m.PrimaryIndex].Cursor
			winCur.ScrollOffset = savedOffset
			CmdEnsureCursorVisible(ctx)
		}
		return true
	}

	if ch == 0 && ev.Key() != tcell.KeyEnter {
		return false
	}

	strToInsert := string(ch)
	if ch == '\t' {
		strToInsert = "\t"
	}

	m.Sort()
	for i := len(m.Cursors) - 1; i >= 0; i-- {
		cur := &m.Cursors[i].Cursor
		line := CursorLine(m.buf, cur)
		if line == nil {
			continue
		}

		pos := cur.Char
		if pos < 0 {
			pos = 0
		}
		if pos > len(line.Value) {
			pos = len(line.Value)
		}

		TextInsert(m.buf, line, pos, strToInsert)

		if ch == '\n' {
			cur.Line++
			cur.Char = 0
			cur.PreserveCharPosition = 0
			for k := i + 1; k < len(m.Cursors); k++ {
				m.Cursors[k].Cursor.Line++
			}
		} else {
			rlen := utf8.RuneCountInString(strToInsert)
			cur.Char += rlen
			cur.PreserveCharPosition = cur.Char
			for k := i + 1; k < len(m.Cursors); k++ {
				if m.Cursors[k].Cursor.Line == cur.Line {
					m.Cursors[k].Cursor.Char += rlen
					m.Cursors[k].Cursor.PreserveCharPosition = m.Cursors[k].Cursor.Char
				}
			}
		}
	}

	if len(m.Cursors) > 0 {
		if m.PrimaryIndex >= len(m.Cursors) || m.PrimaryIndex < 0 {
			m.PrimaryIndex = len(m.Cursors) - 1
		}
		winCur := ContextCursorGet(ctx)
		savedOffset := winCur.ScrollOffset
		*winCur = m.Cursors[m.PrimaryIndex].Cursor
		winCur.ScrollOffset = savedOffset
		CmdEnsureCursorVisible(ctx)
	}
	return true
}

func (m *MultiCursor) RotateForward(ctx Context) {
	if len(m.Cursors) <= 1 {
		return
	}
	m.PrimaryIndex = (m.PrimaryIndex + 1) % len(m.Cursors)
	cur := ContextCursorGet(ctx)
	*cur = m.Cursors[m.PrimaryIndex].Cursor
	if m.Cursors[m.PrimaryIndex].Selection != nil {
		ctx.Buf.Selection = m.Cursors[m.PrimaryIndex].Selection
	}
	CmdCursorCenter(ctx)
	m.Cursors[m.PrimaryIndex].Cursor.ScrollOffset = cur.ScrollOffset
	ctx.Editor.EchoMessage(fmt.Sprintf("%d/%d selections", m.PrimaryIndex+1, len(m.Cursors)))
}

func (m *MultiCursor) AddCursorDown(ctx Context) {
	cur := ContextCursorGet(ctx)
	if cur == nil || ctx.Buf == nil {
		return
	}

	count := max(int(ctx.Count), 1)

	// If a multi-line selection exists, convert it into multi-cursors across the selected lines.
	if ctx.Buf.Selection != nil {
		sel := SelectionNormalize(ctx.Buf.Selection)
		if sel.Start.Line != sel.End.Line {
			m.Clear()
			for lineNum := sel.Start.Line; lineNum <= sel.End.Line; lineNum++ {
				lineElem := CursorLineByNum(ctx.Buf, lineNum)
				maxCh := 0
				var lineRunes []rune
				if lineElem != nil && len(lineElem.Value) > 0 {
					lineRunes = lineElem.Value
					maxCh = max(len(lineElem.Value)-1, 0)
				}
				targetCh := min(RuneIndexFromVisualCol(lineRunes, cur.PreserveCharPosition), maxCh)
				newCur := Cursor{
					Line:                 lineNum,
					Char:                 targetCh,
					PreserveCharPosition: cur.PreserveCharPosition,
				}
				newSel := &Selection{
					Start: Cursor{Line: lineNum, Char: targetCh},
					End:   Cursor{Line: lineNum, Char: targetCh},
				}
				m.Cursors = append(m.Cursors, CursorInstance{Cursor: newCur, Selection: newSel})
			}
			m.PrimaryIndex = len(m.Cursors) - 1
			last := m.Cursors[m.PrimaryIndex]
			ctx.Buf.Selection = last.Selection
			setBufferMode(ctx, MODE_VISUAL)
			*cur = last.Cursor
			CmdEnsureCursorVisible(ctx)
			ctx.Editor.EchoMessage(fmt.Sprintf("%d selections", len(m.Cursors)))
			return
		}
	}

	// If multi-cursor is not yet active, add the current cursor as the base instance.
	if len(m.Cursors) == 0 {
		if ctx.Buf.Selection != nil {
			sel := SelectionNormalize(ctx.Buf.Selection)
			curCopy := *cur
			m.Cursors = append(m.Cursors, CursorInstance{
				Cursor:    curCopy,
				Selection: &sel,
			})
		} else {
			lineElem := CursorLine(ctx.Buf, cur)
			maxCh := 0
			var lineRunes []rune
			if lineElem != nil && len(lineElem.Value) > 0 {
				lineRunes = lineElem.Value
				maxCh = max(len(lineElem.Value)-1, 0)
			}
			charPos := min(cur.Char, maxCh)
			curCopy := *cur
			curCopy.Char = charPos
			curCopy.PreserveCharPosition = VisualCol(lineRunes, cur.Char)
			sel := Selection{
				Start: Cursor{Line: cur.Line, Char: charPos},
				End:   Cursor{Line: cur.Line, Char: charPos},
			}
			m.Cursors = append(m.Cursors, CursorInstance{
				Cursor:    curCopy,
				Selection: &sel,
			})
		}
		m.PrimaryIndex = 0
	}

	for c := 0; c < count; c++ {
		maxLine := -1
		var baseCursor CursorInstance
		for _, ci := range m.Cursors {
			if ci.Cursor.Line > maxLine {
				maxLine = ci.Cursor.Line
				baseCursor = ci
			}
		}

		nextLine := maxLine + 1
		if nextLine >= ctx.Buf.Lines.Len {
			ctx.Editor.EchoMessage("no more lines")
			break
		}

		lineElem := CursorLineByNum(ctx.Buf, nextLine)
		maxCh := 0
		var nextRunes []rune
		if lineElem != nil && len(lineElem.Value) > 0 {
			nextRunes = lineElem.Value
			maxCh = max(len(lineElem.Value)-1, 0)
		}

		visTarget := baseCursor.Cursor.PreserveCharPosition
		if visTarget == 0 && baseCursor.Cursor.Char > 0 {
			baseLineElem := CursorLineByNum(ctx.Buf, baseCursor.Cursor.Line)
			if baseLineElem != nil {
				visTarget = VisualCol(baseLineElem.Value, baseCursor.Cursor.Char)
			}
		}
		newCh := min(RuneIndexFromVisualCol(nextRunes, visTarget), maxCh)

		var newSel *Selection
		if baseCursor.Selection != nil {
			sel := SelectionNormalize(baseCursor.Selection)
			baseLineElem := CursorLineByNum(ctx.Buf, baseCursor.Cursor.Line)
			var baseRunes []rune
			if baseLineElem != nil {
				baseRunes = baseLineElem.Value
			}
			startVis := VisualCol(baseRunes, sel.Start.Char)
			endVis := VisualCol(baseRunes, sel.End.Char)
			startCh := min(RuneIndexFromVisualCol(nextRunes, startVis), maxCh)
			endCh := min(RuneIndexFromVisualCol(nextRunes, endVis), maxCh)
			newSel = &Selection{
				Start: Cursor{Line: nextLine, Char: startCh},
				End:   Cursor{Line: nextLine, Char: endCh},
			}
			newCh = min(RuneIndexFromVisualCol(nextRunes, VisualCol(baseRunes, baseCursor.Cursor.Char)), maxCh)
		} else {
			newSel = &Selection{
				Start: Cursor{Line: nextLine, Char: newCh},
				End:   Cursor{Line: nextLine, Char: newCh},
			}
		}

		newCur := Cursor{
			Line:                 nextLine,
			Char:                 newCh,
			PreserveCharPosition: visTarget,
		}
		m.Cursors = append(m.Cursors, CursorInstance{
			Cursor:    newCur,
			Selection: newSel,
		})
	}

	m.PrimaryIndex = len(m.Cursors) - 1
	last := m.Cursors[m.PrimaryIndex]
	if last.Selection != nil {
		ctx.Buf.Selection = last.Selection
		setBufferMode(ctx, MODE_VISUAL)
	}
	*cur = last.Cursor
	CmdEnsureCursorVisible(ctx)
	ctx.Editor.EchoMessage(fmt.Sprintf("%d selections", len(m.Cursors)))
}

func (m *MultiCursor) RotateBackward(ctx Context) {
	if len(m.Cursors) <= 1 {
		return
	}
	m.PrimaryIndex = (m.PrimaryIndex - 1 + len(m.Cursors)) % len(m.Cursors)
	cur := ContextCursorGet(ctx)
	*cur = m.Cursors[m.PrimaryIndex].Cursor
	if m.Cursors[m.PrimaryIndex].Selection != nil {
		ctx.Buf.Selection = m.Cursors[m.PrimaryIndex].Selection
	}
	CmdCursorCenter(ctx)
	m.Cursors[m.PrimaryIndex].Cursor.ScrollOffset = cur.ScrollOffset
	ctx.Editor.EchoMessage(fmt.Sprintf("%d/%d selections", m.PrimaryIndex+1, len(m.Cursors)))
}

func indexOfRunes(runes, pat []rune) int {
	if len(pat) == 0 || len(runes) < len(pat) {
		return -1
	}
	for i := 0; i <= len(runes)-len(pat); i++ {
		match := true
		for j := 0; j < len(pat); j++ {
			if runes[i+j] != pat[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
