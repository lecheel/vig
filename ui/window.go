package ui

import (
	"fmt"
	"strings"

	str "github.com/boyter/go-string"
	"github.com/mattn/go-runewidth"

	"github.com/firstrow/wig"
)

func WindowRender(e *wig.Editor, view wig.View, win *wig.Window) {
	buf := win.Buffer()
	if buf == nil {
		return
	}
	cur := wig.WindowCursorGet(win, buf)

	termWidth, termHeight := view.Size()
	termWidth -= 2
	termHeight -= 1

	currentLine := buf.Lines.First()
	offset := cur.ScrollOffset

	// Line numbers config
	lineNum := 0
	lineNumTextStyle := wig.Color("ui.linenr")
	lineNumTextStyleSelected := wig.Color("ui.linenr.selected")
	hasGitSigns := true
	leftPadding := WindowTextPadding(e, buf)
	signColWidth := 2
	lineNumWidth := 0
	if e.Config.ShowLineNumbers {
		lineNumWidth = len(fmt.Sprintf("%d", buf.CountLines())) + 1
	}
	blameColWidth := 0
	if buf.BlameEnabled && len(buf.BlameLines) > 0 {
		blameColWidth = buf.BlameWidth + 1
	}

	y := 0

	isActiveWin := win == e.ActiveWindow()

	textWidth := max(termWidth-leftPadding, 1)
	skip := 0
	if cur.Char >= textWidth {
		skip = cur.Char - textWidth + 1
	}

	if buf.Highlighter == nil && buf.FilePath != "" {
		buf.Highlighter = wig.NewHighlighterForPath(buf, buf.FilePath)
	}

	// Precalculate visual block bounds for efficient rendering
	// Block-selection bounds are fixed for the whole render pass — compute
	// them once here instead of re-deriving minLine/maxLine from
	// buf.Selection on every character (as the per-character code below
	// used to do, in three separate places).
	var minVisCol, maxVisCol int
	var blockMinLine, blockMaxLine int
	isVisualBlock := buf.Mode() == wig.MODE_VISUAL_BLOCK && buf.Selection != nil
	if isVisualBlock {
		sel := buf.Selection
		startLineNode := wig.CursorLineByNum(buf, sel.Start.Line)
		endLineNode := wig.CursorLineByNum(buf, sel.End.Line)
		startVisCol := wig.VisualCol(startLineNode.Value, sel.Start.Char)
		endVisCol := wig.VisualCol(endLineNode.Value, sel.End.Char)
		minVisCol = min(startVisCol, endVisCol)
		maxVisCol = max(startVisCol, endVisCol)
		blockMinLine, blockMaxLine = sel.Start.Line, sel.End.Line
		if blockMinLine > blockMaxLine {
			blockMinLine, blockMaxLine = blockMaxLine, blockMinLine
		}
	}

	for currentLine != nil {
		if lineNum >= offset && y <= termHeight {
			// render each character in the line separately
			x := leftPadding // onscreen position

			isLineInBlockRange := isVisualBlock && lineNum >= blockMinLine && lineNum <= blockMaxLine

			// highlight search
			searchMatches := [][]int{}
			if wig.LastSearchPattern != "" {
				searchMatches = str.IndexAllIgnoreCase(string(currentLine.Value), wig.LastSearchPattern, -1)
			}

			diagnostics := e.Lsp.Diagnostics(buf, lineNum)

			// Line numbers & Git Signs & Marks & Blame
			if e.Config.ShowLineNumbers || hasGitSigns || buf.BlameEnabled {
				xCur := 0
				if hasGitSigns {
					sign, ok := buf.GitSigns[lineNum+1] // GitSigns is 1-indexed
					signStyle := wig.Color("default")

					// Check for mark on this line
					markChar := " "
					markStyle := wig.Color("ui.linenr")
					if e.Marks != nil {
						for r, m := range e.Marks {
							if m.Buf == buf && m.Cursor.Line == lineNum {
								markChar = string(r)
								markStyle = wig.Color("ui.mark")
								break
							}
						}
					}

					if xCur >= 0 && xCur < termWidth && y >= 0 && y < termHeight {
						// Draw git sign in first col
						if ok {
							if sign == '+' {
								signStyle = wig.Color("diff.plus")
							} else if sign == '-' {
								signStyle = wig.Color("diff.minus")
							} else if sign == '~' {
								signStyle = wig.Color("diff.delta")
							}
							view.SetContent(xCur, y, string(sign), signStyle)
						} else {
							view.SetContent(xCur, y, " ", signStyle)
						}
						// Draw mark character in second col
						view.SetContent(xCur+1, y, markChar, markStyle)
					}
					xCur += signColWidth
				}

				if e.Config.ShowLineNumbers {
					lnNum := lineNum + 1
					if e.Config.RelativeLineNumbers {
						lnNum = cur.Line - lineNum
						if lnNum < 0 {
							lnNum = -lnNum
						}
						// If hybrid mode is enabled, show absolute line number on current line
						if e.Config.CurrentLineAbsolute && lineNum == cur.Line {
							lnNum = lineNum + 1
						}
					}

					if xCur >= 0 && xCur < termWidth && y >= 0 && y < termHeight {
						if lineNum == cur.Line {
							view.SetContent(xCur, y, fmt.Sprintf("%d", lnNum), lineNumTextStyleSelected)
						} else {
							view.SetContent(xCur, y, fmt.Sprintf("%d", lnNum), lineNumTextStyle)
						}
					}
				}
				xCur += lineNumWidth

				if buf.BlameEnabled && blameColWidth > 0 {
					if info, ok := buf.BlameLines[lineNum]; ok {
						if xCur >= 0 && xCur < termWidth && y >= 0 && y < termHeight {
							view.SetContent(xCur, y, info.Display, wig.Color("comment"))
						}
					}
					xCur += blameColWidth
				}
			}
			// End Line Numbers & Git Signs

			// highlight current line background
			if lineNum == cur.Line {
				fillWidth := termWidth - leftPadding
				if fillWidth > 0 {
					cursorLineBg := wig.ApplyBg("ui.cursorline", wig.Color("default"))
					bg := strings.Repeat(" ", fillWidth)
					view.SetContent(leftPadding, y, bg, cursorLineBg)
				}
			}

			// render line
			startVisCol := 0
			for j := 0; j < skip && j < len(currentLine.Value); j++ {
				startVisCol += chlen(currentLine.Value[j])
			}
			currVisCol := startVisCol

			var lineSpans []wig.Span
			if buf.Highlighter != nil {
				lineSpans = buf.Highlighter.HighlightLine(lineNum)
			}
			spanIdx := 0

			for i := skip; i < len(currentLine.Value); i++ {
				// render selection
				textStyle := wig.Color("default")

				for spanIdx < len(lineSpans) && int(lineSpans[spanIdx].EndCol) <= i {
					spanIdx++
				}
				if spanIdx < len(lineSpans) && int(lineSpans[spanIdx].StartCol) <= i && int(lineSpans[spanIdx].EndCol) > i {
					textStyle = lineSpans[spanIdx].Style
				}

				// Colors and styles
				charLen := chlen(currentLine.Value[i])

				// highlight current line
				if lineNum == cur.Line {
					textStyle = wig.ApplyBg("ui.cursorline", textStyle)
				}

				// selection
				if buf.Selection != nil {
					if isVisualBlock {
						// Tabs are excluded here and handled per-column
						// below — a tab is one rune spanning 4 columns,
						// so this whole-rune check would force the
						// entire tab in/out as a single unit instead of
						// only the columns the block actually covers.
						if isLineInBlockRange && currentLine.Value[i] != '\t' && currVisCol < maxVisCol && currVisCol+charLen > minVisCol {
							textStyle = wig.ApplyBg("ui.selection.primary", textStyle)
						}
					} else if wig.SelectionCursorInRange(buf.Selection, wig.Cursor{Line: lineNum, Char: i}) {
						textStyle = wig.ApplyBg("ui.selection.primary", textStyle)
					}
				}

				// highlight search
				if len(searchMatches) > 0 {
					matchLen := len(wig.LastSearchPattern)
					for _, m := range searchMatches {
						if len(m) > 0 {
							start := m[0]
							end := start + matchLen
							if len(m) > 1 {
								end = m[1]
							}
							if i >= start && i < end {
								textStyle = wig.ApplyBg("ui.selection", textStyle)
							}
						}
					}
				}

				// lsp errors
				if len(diagnostics) > 0 {
					for _, info := range diagnostics {
						if i >= int(info.Range.Start.Character) && i < int(info.Range.End.Character) {
							textStyle = wig.MergeStyles(textStyle, "diagnostic.error")
						}
						// once character error
						if info.Range.Start.Character == info.Range.End.Character && info.Range.End.Character == uint32(i) {
							textStyle = wig.MergeStyles(textStyle, "diagnostic.error")
						}
					}
				}

				/////////////////////////////////

				if i > len(currentLine.Value)-1 {
					continue
				}

				ch := getRenderChar(currentLine.Value[i])

				// todo: handle tabs colors?
				// render text
				//
				// A tab is a single rune but occupies charLen (4) visual
				// columns. In blockwise (Ctrl-V) selection, a boundary can
				// fall in the middle of that span — rendering it as one
				// SetContent call can only highlight the whole tab or none
				// of it. Split it into per-column cells here so only the
				// columns actually inside [minVisCol,maxVisCol) light up.
				// Only needed when this line is actually part of the block
				// and the char is a tab; otherwise render normally.
				if currentLine.Value[i] == '\t' && isLineInBlockRange {
					for col := 0; col < charLen; col++ {
						cellStyle := textStyle
						cellVisCol := currVisCol + col
						if cellVisCol >= minVisCol && cellVisCol < maxVisCol {
							cellStyle = wig.ApplyBg("ui.selection.primary", cellStyle)
						}
						cellX := x + col
						if cellX >= 0 && cellX < termWidth && y >= 0 && y < termHeight {
							view.SetContent(cellX, y, " ", cellStyle)
						}
					}
				} else if x >= 0 && x < termWidth && y >= 0 && y < termHeight {
					view.SetContent(x, y, string(ch), textStyle)
				}

				// render cursor
				if isActiveWin {
					if lineNum == cur.Line && i == cur.Char {
						baseCursor := wig.Color("default")
						if c, found := wig.FindColor("ui.selection"); found {
							baseCursor = c
						}
						if c, found := wig.FindColor("ui.cursor"); found {
							baseCursor = c
						}
						if buf.Mode() == wig.MODE_INSERT {
							if c, found := wig.FindColor("ui.cursor.primary.insert"); found {
								baseCursor = c
							}
						}
						if buf.Mode() == wig.MODE_VISUAL {
							if c, found := wig.FindColor("ui.cursor.primary.select"); found {
								baseCursor = c
							}
						}
						if x >= 0 && x < termWidth && y >= 0 && y < termHeight {
							view.SetContent(x, y, ch, baseCursor)
						}
					}
				}

				x += charLen
				currVisCol += charLen
			}

			// render cursor after the end of the line in insert mode
			if isLineInBlockRange && currVisCol < maxVisCol {
				selStyle := wig.ApplyBg("ui.selection.primary", wig.Color("default"))
				startPad := currVisCol
				if startPad < minVisCol {
					startPad = minVisCol
				}
				if startPad < startVisCol {
					startPad = startVisCol
				}
				for visCol := startPad; visCol < maxVisCol; visCol++ {
					padX := leftPadding + visCol - startVisCol
					if padX >= 0 && padX < termWidth && y >= 0 && y < termHeight {
						view.SetContent(padX, y, " ", selStyle)
					}
				}
			}

			if lineNum == cur.Line && cur.Char >= len(currentLine.Value) && isActiveWin {
				if x >= 0 && x < termWidth && y >= 0 && y < termHeight {
					view.SetContent(x, y, " ", wig.Color("ui.cursor"))
				}
			}
			y++
		}

		currentLine = currentLine.Next()
		lineNum++
	}
}

func chlen(c rune) int {
	if c == '\t' {
		return 4
	}
	if c == '\n' {
		return 0
	}
	return runewidth.RuneWidth(c)
}

func getRenderChar(c rune) string {
	if c == '\t' {
		return "    "
	}
	if c == '\n' {
		return " "
	}
	return string(c)
}

// WindowTextPadding returns the X offset at which buffer text begins inside
// a window's viewport, accounting for the git sign column, line-number
// gutter, and optional blame column.
//
// Every renderer (text in WindowRender, indent guides in
// render.RenderIndentGuides, future overlays like diagnostics / virtual
// text) MUST use this function. Hardcoding the layout in two places is
// what previously caused indent guides to drift when the git-sign column
// was always reserved by WindowRender (hasGitSigns := true) but
// RenderIndentGuides still used len(buf.GitSigns) > 0.
func WindowTextPadding(e *wig.Editor, buf *wig.Buffer) int {
	// Always reserve the sign column so the gutter width is stable
	// whether or not the current buffer currently has git signs. This
	// matches WindowRender's hasGitSigns := true behaviour.
	signColWidth := 2

	lineNumWidth := 0
	if e.Config.ShowLineNumbers {
		lineNumWidth = len(fmt.Sprintf("%d", buf.CountLines())) + 1
	}

	blameColWidth := 0
	if buf.BlameEnabled && len(buf.BlameLines) > 0 {
		blameColWidth = buf.BlameWidth + 1
	}

	return signColWidth + lineNumWidth + blameColWidth
}
