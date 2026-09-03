package ui

import (
	"fmt"
	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var cmdHistory []string

type uiCommandLine struct {
	e                 *wig.Editor
	keymap            *wig.KeyHandler
	chBuf             []rune
	cursorPos         int
	historyIdx        int
	candidates        []string
	candIdx           int
	ctrlRMode         bool
	origSearchPattern string
}

func (u *uiCommandLine) updateSubstitutionHighlight() {
	input := string(u.chBuf)
	rest := input
	if strings.HasPrefix(rest, "%") {
		rest = rest[1:]
	}
	if strings.HasPrefix(rest, "s/") {
		rest = rest[2:]
		pattern := unescapeSearchPattern(splitSubstPattern(rest))
		if wig.LastSearchPattern != pattern {
			wig.LastSearchPattern = pattern
			u.e.Redraw()
		}
	}
}

// splitSubstPattern extracts the pattern portion (between the first and
// second unescaped "/") from the text following "s/" in a :s command,
// preserving backslash-escape pairs intact (e.g. "\/" stays "\/").
func splitSubstPattern(rest string) string {
	pattern := ""
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			pattern += string(rest[i]) + string(rest[i+1])
			i++
			continue
		}
		if rest[i] == '/' {
			break
		}
		pattern += string(rest[i])
	}
	return pattern
}

// unescapeSearchPattern converts a raw regex pattern (with escape
// sequences like \/, \\, \d intact) into a plain display/highlight
// string, by dropping the backslash from each escape pair. This is used
// wherever LastSearchPattern feeds a literal substring matcher (live
// preview highlight, post-substitution n/N search) rather than
// regexp.Compile, which needs the escapes preserved.
func unescapeSearchPattern(pattern string) string {
	var sb strings.Builder
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			sb.WriteByte(pattern[i+1])
			i++
			continue
		}
		sb.WriteByte(pattern[i])
	}
	return sb.String()
}

func (u *uiCommandLine) Plane() wig.RenderPlane {
	return wig.PlaneEditor
}

func CmdLineInit(ctx wig.Context) {
	u := &uiCommandLine{
		e:                 ctx.Editor,
		chBuf:             make([]rune, 0, 32),
		cursorPos:         0,
		historyIdx:        len(cmdHistory),
		candidates:        []string{},
		candIdx:           -1,
		origSearchPattern: wig.LastSearchPattern,
	}

	// Pre-fill range for visual modes, exactly like Vim
	if ctx.Buf != nil && (ctx.Buf.Mode() == wig.MODE_VISUAL || ctx.Buf.Mode() == wig.MODE_VISUAL_LINE || ctx.Buf.Mode() == wig.MODE_VISUAL_BLOCK) {
		u.chBuf = []rune("'<,'>")
		u.cursorPos = len(u.chBuf)
	}
	u.keymap = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_INSERT: wig.KeyMap{
			"Esc": func(ctx wig.Context) {
				wig.LastSearchPattern = u.origSearchPattern
				ctx.Editor.PopUi()
				ctx.Editor.Redraw()
			},
			"Enter": func(ctx wig.Context) {
				cmd := string(u.chBuf)
				if strings.TrimSpace(cmd) != "" {
					cmdHistory = append(cmdHistory, cmd)
				}
				u.execute(cmd)
			},
			"Tab": func(ctx wig.Context) {
				if len(u.candidates) == 0 {
					u.autocomplete()
				} else {
					u.navigateCandidate(1, 0)
				}
			},
			"Up": func(ctx wig.Context) {
				if len(u.candidates) > 0 {
					u.navigateCandidate(0, 1)
					return
				}
				if u.historyIdx > 0 {
					u.historyIdx--
					u.chBuf = []rune(cmdHistory[u.historyIdx])
					u.cursorPos = len(u.chBuf)
					u.candidates = []string{}
					u.candIdx = -1
				}
			},
			"Down": func(ctx wig.Context) {
				if len(u.candidates) > 0 {
					u.navigateCandidate(0, -1)
					return
				}
				if u.historyIdx < len(cmdHistory)-1 {
					u.historyIdx++
					u.chBuf = []rune(cmdHistory[u.historyIdx])
					u.cursorPos = len(u.chBuf)
					u.candidates = []string{}
					u.candIdx = -1
				} else {
					u.historyIdx = len(cmdHistory)
					u.chBuf = []rune{}
					u.cursorPos = 0
					u.candidates = []string{}
					u.candIdx = -1
				}
			},
			"Left": func(ctx wig.Context) {
				if len(u.candidates) > 0 {
					u.navigateCandidate(-1, 0)
					return
				}
				if u.cursorPos > 0 {
					u.cursorPos--
				}
			},
			"Right": func(ctx wig.Context) {
				if len(u.candidates) > 0 {
					u.navigateCandidate(1, 0)
					return
				}
				if u.cursorPos < len(u.chBuf) {
					u.cursorPos++
				}
			},
		},
	})
	u.keymap.Fallback(u.insertCh)
	ctx.Editor.PushUi(u)
}

func (u *uiCommandLine) insertCh(ctx wig.Context, ev *tcell.EventKey) {
	// Handle Ctrl-r special mode
	if u.ctrlRMode {
		u.ctrlRMode = false
		if ev.Key() == tcell.KeyCtrlW {
			// Grab the word under cursor from buffer (letters, digits, underscores, excluding delimiters like .([)
			eCtx := u.e.NewContext()
			if eCtx.Buf != nil {
				cur := wig.ContextCursorGet(eCtx)
				line := wig.CursorLine(eCtx.Buf, cur)
				if line != nil && len(line.Value) > 0 {
					chars := line.Value
					idx := cur.Char
					if idx >= len(chars) {
						idx = len(chars) - 1
					}

					isWordChar := func(r rune) bool {
						return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
					}

					if !isWordChar(chars[idx]) {
						if idx > 0 && isWordChar(chars[idx-1]) {
							idx--
						}
					}

					if isWordChar(chars[idx]) {
						start := idx
						for start > 0 && isWordChar(chars[start-1]) {
							start--
						}

						end := idx
						for end < len(chars) && isWordChar(chars[end]) {
							end++
						}

						if end > start {
							word := string(chars[start:end])
							wordRunes := []rune(word)

							newBuf := make([]rune, len(u.chBuf)+len(wordRunes))
							copy(newBuf, u.chBuf[:u.cursorPos])
							copy(newBuf[u.cursorPos:], wordRunes)
							copy(newBuf[u.cursorPos+len(wordRunes):], u.chBuf[u.cursorPos:])

							u.chBuf = newBuf
							u.cursorPos += len(wordRunes)
							u.candidates = []string{}
							u.candIdx = -1
						}
					}
				}
			}
			return
		}

		// Register insertion: <Ctrl-r>{reg} (e.g. +, %, 0-9, a-z, ")
		var regKey rune
		if ev.Key() == tcell.KeyRune {
			regKey = ev.Rune()
		}
		if regKey != 0 {
			eCtx := u.e.NewContext()
			text := wig.GetRegisterText(eCtx, regKey)
			if text != "" {
				textRunes := []rune(text)
				newBuf := make([]rune, len(u.chBuf)+len(textRunes))
				copy(newBuf, u.chBuf[:u.cursorPos])
				copy(newBuf[u.cursorPos:], textRunes)
				copy(newBuf[u.cursorPos+len(textRunes):], u.chBuf[u.cursorPos:])

				u.chBuf = newBuf
				u.cursorPos += len(textRunes)
				u.candidates = []string{}
				u.candIdx = -1
			}
			return
		}
		// If not handled, fall through and process normally
	}

	if ev.Modifiers()&tcell.ModCtrl != 0 {
		switch ev.Key() {
		case tcell.KeyCtrlR:
			u.ctrlRMode = true
		case tcell.KeyCtrlA:
			u.cursorPos = 0
		case tcell.KeyCtrlE:
			u.cursorPos = len(u.chBuf)
		case tcell.KeyCtrlU:
			u.chBuf = u.chBuf[u.cursorPos:]
			u.cursorPos = 0
			u.candidates = []string{}
			u.candIdx = -1
			u.updateSubstitutionHighlight()
		case tcell.KeyCtrlK:
			u.chBuf = u.chBuf[:u.cursorPos]
			u.candidates = []string{}
			u.candIdx = -1
			u.updateSubstitutionHighlight()
		case tcell.KeyCtrlW:
			if u.cursorPos == 0 {
				return
			}
			start := u.cursorPos
			for start > 0 && u.chBuf[start-1] == ' ' {
				start--
			}
			for start > 0 && u.chBuf[start-1] != ' ' {
				start--
			}
			u.chBuf = append(u.chBuf[:start], u.chBuf[u.cursorPos:]...)
			u.cursorPos = start
			u.candidates = []string{}
			u.candIdx = -1
			u.updateSubstitutionHighlight()
		}
		return
	}

	if ev.Modifiers()&tcell.ModAlt != 0 || ev.Modifiers()&tcell.ModMeta != 0 {
		return
	}

	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if u.cursorPos > 0 {
			u.chBuf = append(u.chBuf[:u.cursorPos-1], u.chBuf[u.cursorPos:]...)
			u.cursorPos--
			u.candidates = []string{}
			u.candIdx = -1
			u.updateSubstitutionHighlight()
		}
		return
	case tcell.KeyDelete:
		if u.cursorPos < len(u.chBuf) {
			u.chBuf = append(u.chBuf[:u.cursorPos], u.chBuf[u.cursorPos+1:]...)
			u.candidates = []string{}
			u.candIdx = -1
			u.updateSubstitutionHighlight()
		}
		return
	case tcell.KeyLeft:
		if u.cursorPos > 0 {
			u.cursorPos--
		}
		return
	case tcell.KeyRight:
		if u.cursorPos < len(u.chBuf) {
			u.cursorPos++
		}
		return
	case tcell.KeyHome:
		u.cursorPos = 0
		return
	case tcell.KeyEnd:
		u.cursorPos = len(u.chBuf)
		return
	case tcell.KeyRune:
		u.chBuf = append(u.chBuf, 0)
		copy(u.chBuf[u.cursorPos+1:], u.chBuf[u.cursorPos:])
		u.chBuf[u.cursorPos] = ev.Rune()
		u.cursorPos++
		u.candidates = []string{}
		u.candIdx = -1
		u.updateSubstitutionHighlight()
		return
	}
}

func (u *uiCommandLine) navigateCandidate(dx, dy int) {
	if len(u.candidates) == 0 {
		return
	}

	// Calculate grid dimensions matching Render logic
	maxLen := 0
	for _, c := range u.candidates {
		if len([]rune(c)) > maxLen {
			maxLen = len([]rune(c))
		}
	}
	minColWidth := maxLen + 2
	if minColWidth < 10 {
		minColWidth = 10
	}
	vw, _ := u.e.View.Size()
	cols := vw / minColWidth
	if cols == 0 {
		cols = 1
	}
	if cols > len(u.candidates) {
		cols = len(u.candidates)
	}

	rows := (len(u.candidates) + cols - 1) / cols

	// If no current selection, start from top-left or bottom-right depending on direction
	if u.candIdx == -1 {
		if dy > 0 || dx > 0 {
			u.candIdx = 0
		} else {
			u.candIdx = len(u.candidates) - 1
		}
	} else {
		curRow := u.candIdx / cols
		curCol := u.candIdx % cols

		newCol := curCol + dx
		newRow := curRow + dy

		if newCol >= cols {
			newCol = 0
			newRow++
		}
		if newCol < 0 {
			newCol = cols - 1
			newRow--
		}

		if newRow >= rows {
			newRow = 0
		}
		if newRow < 0 {
			newRow = rows - 1
		}

		newIdx := newRow*cols + newCol
		if newIdx >= len(u.candidates) {
			if dx != 0 {
				newIdx = 0
			} else {
				newIdx = len(u.candidates) - 1
			}
		}
		u.candIdx = newIdx
	}

	input := string(u.chBuf)
	parts := strings.SplitN(input, " ", 2)
	if len(parts) == 2 && (parts[0] == "e" || parts[0] == "edit") {
		u.chBuf = []rune(fmt.Sprintf("%s %s", parts[0], u.candidates[u.candIdx]))
	} else {
		u.chBuf = []rune(u.candidates[u.candIdx])
	}
	u.cursorPos = len(u.chBuf)
}

func (u *uiCommandLine) autocomplete() {
	input := string(u.chBuf)
	parts := strings.SplitN(input, " ", 2)

	// File path completion for :e or :edit
	if len(parts) == 2 && (parts[0] == "e" || parts[0] == "edit") {
		cmdPart := parts[0]
		prefix := parts[1]

		if len(u.candidates) > 0 {
			u.navigateCandidate(1, 0)
			return
		}

		dir := "."
		filePrefix := prefix
		if strings.Contains(prefix, "/") {
			lastSlash := strings.LastIndex(prefix, "/")
			dir = prefix[:lastSlash]
			if dir == "" {
				dir = "/"
			}
			filePrefix = prefix[lastSlash+1:]
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		var matches []string
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, filePrefix) {
				if entry.IsDir() {
					name += "/"
				}
				if dir != "." {
					if dir == "/" {
						name = "/" + name
					} else {
						name = dir + "/" + name
					}
				}
				matches = append(matches, name)
			}
		}

		if len(matches) == 0 {
			return
		}

		sort.Strings(matches)

		common := matches[0]
		for _, m := range matches[1:] {
			i := 0
			for i < len(common) && i < len(m) && common[i] == m[i] {
				i++
			}
			common = common[:i]
		}

		u.chBuf = []rune(fmt.Sprintf("%s %s", cmdPart, common))
		u.cursorPos = len(u.chBuf)
		u.candidates = matches
		u.candIdx = -1 // Next Tab will cycle to 0
		return
	}

	// Command name completion
	prefix := input
	if len(u.candidates) > 0 {
		u.navigateCandidate(1, 0)
		return
	}

	var matches []string
	for name := range wig.AllCommands {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return
	}

	sort.Strings(matches)

	if len(matches) == 1 {
		u.chBuf = []rune(matches[0])
		u.cursorPos = len(u.chBuf)
		u.candidates = matches
		u.candIdx = 0
		return
	}

	common := matches[0]
	for _, m := range matches[1:] {
		i := 0
		for i < len(common) && i < len(m) && common[i] == m[i] {
			i++
		}
		common = common[:i]
	}
	u.chBuf = []rune(common)
	u.cursorPos = len(u.chBuf)
	u.candidates = matches
	u.candIdx = -1
}

func (u *uiCommandLine) execute(cmd string) {
	u.runCommand(cmd)
}

func (u *uiCommandLine) runCommand(cmd string) {
	u.e.PopUiComponent(u)

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	wig.LastCommand = cmd

	// Handle ranges like '<,'> or %
	rangeStr := ""
	restCmd := cmd
	if strings.HasPrefix(cmd, "%") {
		rangeStr = "%"
		restCmd = cmd[1:]
	} else if strings.HasPrefix(cmd, "'<,'>") {
		rangeStr = "'<,'>"
		restCmd = cmd[5:]
	}

	// If a visual range was used, ensure we exit visual mode after the command runs.
	// This matches Vim's behavior where executing an Ex command drops you back to Normal mode.
	if rangeStr == "'<,'>" {
		defer func() {
			ctx := u.e.NewContext()
			if ctx.Buf != nil {
				wig.CmdNormalMode(ctx)
			}
		}()
	}

	// Jump to line number (e.g. :123)
	if lineNum, err := strconv.Atoi(restCmd); err == nil {
		ctx := u.e.NewContext()
		ctx.Count = uint32(lineNum)
		wig.CmdGotoLine0(ctx)
		return
	}

	parts := strings.SplitN(restCmd, " ", 2)
	if len(parts) > 0 && (parts[0] == "e" || parts[0] == "edit") {
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			u.e.EchoMessage("No file name")
			return
		}
		filePath := strings.TrimSpace(parts[1])
		ctx := u.e.NewContext()
		if filePath == "%" && ctx.Buf != nil {
			if def, ok := wig.AllCommands["CmdReloadBuffer"]; ok {
				if fn, ok := def.Fn.(func(wig.Context)); ok {
					fn(ctx)
					return
				}
			}
			filePath = ctx.Buf.FilePath
		}
		buf, err := u.e.OpenFile(filePath)
		if err != nil {
			u.e.EchoMessage(fmt.Sprintf("Error opening %s: %v", filePath, err))
			return
		}
		ctx.Buf = buf
		u.e.ActiveWindow().VisitBuffer(ctx)
		return
	}

	// Search and replace: :s/pat/rep/flags, :%s/pat/rep/flags, or :'<,'>s/pat/rep/flags
	isGlobal := rangeStr == "%"
	if strings.HasPrefix(restCmd, "s/") {
		replaceCmd := restCmd[2:]
		var parts []string
		current := strings.Builder{}
		for i := 0; i < len(replaceCmd); i++ {
			if len(parts) >= 2 {
				current.WriteString(replaceCmd[i:])
				break
			}
			if replaceCmd[i] == '\\' && i+1 < len(replaceCmd) {
				current.WriteByte(replaceCmd[i])
				current.WriteByte(replaceCmd[i+1])
				i++
				continue
			}
			if replaceCmd[i] == '/' {
				parts = append(parts, current.String())
				current.Reset()
				continue
			}
			current.WriteByte(replaceCmd[i])
		}
		parts = append(parts, current.String())
		if len(parts) < 2 {
			u.e.EchoMessage("Invalid search command")
			return
		}

		pattern := parts[0]
		replacement := parts[1]
		flags := ""
		if len(parts) == 3 {
			flags = parts[2]
		}

		ctx := u.e.NewContext()
		buf := ctx.Buf

		startLine := 0
		endLine := buf.Lines.Len - 1
		if !isGlobal {
			if buf.Selection != nil {
				sel := wig.SelectionNormalize(buf.Selection)
				startLine = sel.Start.Line
				endLine = sel.End.Line
			} else {
				cur := wig.ContextCursorGet(ctx)
				startLine = cur.Line
				endLine = cur.Line
			}
		}

		replaceAll := strings.Contains(flags, "g")
		confirm := strings.Contains(flags, "c")

		re, err := regexp.Compile(pattern)
		if err != nil {
			wig.LastSearchPattern = u.origSearchPattern
			u.e.EchoMessage("Invalid regex: " + err.Error())
			return
		}
		// Compile with the raw/escaped pattern (needed for correct regex
		// semantics), but store the unescaped display form for highlight
		// matching, which is a plain substring search, not a regex engine.
		wig.LastSearchPattern = unescapeSearchPattern(pattern)

		if !confirm {
			if buf.TxStart() {
				defer buf.TxEnd()
			}
			count := 0
			for i := startLine; i <= endLine; i++ {
				line := wig.CursorLineByNum(buf, i)
				lineRunes := line.Value
				lineLen := len(lineRunes) - 1
				text := string(lineRunes)

				var newText string
				if replaceAll {
					newText = re.ReplaceAllString(text, replacement)
					count += len(re.FindAllString(text, -1))
				} else {
					loc := re.FindStringSubmatchIndex(text)
					if loc != nil {
						match := text[loc[0]:loc[1]]
						expanded := re.ReplaceAllString(match, replacement)
						newText = text[:loc[0]] + expanded + text[loc[1]:]
						count++
					} else {
						newText = text
					}
				}

				if newText != text {
					wig.TextDelete(buf, &wig.Selection{
						Start: wig.Cursor{Line: i, Char: 0},
						End:   wig.Cursor{Line: i, Char: lineLen},
					})
					wig.TextInsert(buf, line, 0, newText[:len(newText)-1])
				}
			}
			u.e.EchoMessage(fmt.Sprintf("%d substitutions", count))
		} else {
			// With confirmation
			if buf.TxStart() {
				defer buf.TxEnd()
			}

			i := startLine
			offset := 0
			var processNext func()
			processNext = func() {
				if i > endLine {
					u.e.EchoMessage("substitutions done")
					return
				}

				line := wig.CursorLineByNum(buf, i)
				text := string(line.Value)
				// search from offset
				loc := re.FindStringSubmatchIndex(text[offset:])
				if loc == nil {
					i++
					offset = 0
					processNext()
					return
				}

				actualStart := offset + loc[0]
				actualEnd := offset + loc[1]

				cur := wig.ContextCursorGet(ctx)
				cur.Line = i
				cur.Char = utf8.RuneCountInString(text[:actualStart])
				ctx.Editor.Redraw()

				matchStr := text[actualStart:actualEnd]
				expandedStr := re.ReplaceAllString(matchStr, replacement)

				prompt := fmt.Sprintf("Replace '%s' with '%s'? (y/n)", matchStr, expandedStr)
				ConfirmInit(ctx, prompt, func() {
					// Yes
					newText := text[:actualStart] + expandedStr + text[actualEnd:]
					lineLen := len(line.Value) - 1
					wig.TextDelete(buf, &wig.Selection{
						Start: wig.Cursor{Line: i, Char: 0},
						End:   wig.Cursor{Line: i, Char: lineLen},
					})
					wig.TextInsert(buf, line, 0, newText[:len(newText)-1])

					// advance offset by length of expandedStr
					offset = actualStart + len(expandedStr)
					if !replaceAll {
						i++
						offset = 0
					}
					processNext()
				}, func() {
					// No
					offset = actualEnd
					if !replaceAll {
						i++
						offset = 0
					}
					processNext()
				}, func() {
					// Cancel
					u.e.EchoMessage("Replace cancelled")
				})
			}
			processNext()
		}
		return
	}

	cmdName := restCmd
	arg := ""
	if spIdx := strings.Index(restCmd, " "); spIdx != -1 {
		cmdName = restCmd[:spIdx]
		arg = strings.TrimSpace(restCmd[spIdx+1:])
	}

	if def, ok := wig.AllCommands[restCmd]; ok {
		ctx := u.e.NewContext()
		if fn, ok := def.Fn.(func(wig.Context)); ok {
			if def.Repeatable {
				u.e.LastRepeatableFn = fn
			}
			fn(ctx)
		} else {
			u.e.EchoMessage(fmt.Sprintf("Command %s is not executable", restCmd))
		}
	} else if def, ok := wig.AllCommands[cmdName]; ok {
		ctx := u.e.NewContext()
		ctx.Char = arg
		if fn, ok := def.Fn.(func(wig.Context)); ok {
			if def.Repeatable {
				u.e.LastRepeatableFn = fn
			}
			fn(ctx)
		} else {
			u.e.EchoMessage(fmt.Sprintf("Command %s is not executable", cmdName))
		}
	} else {
		u.e.EchoMessage(fmt.Sprintf("Unknown command: %s", restCmd))
	}
}

func (u *uiCommandLine) Keymap() *wig.KeyHandler {
	return u.keymap
}

func (u *uiCommandLine) Render(view wig.View) {
	vw, vh := view.Size()
	y := vh - 2
	if y < 0 {
		return
	}
	promptPrefix := ":"

	beforeCursor := string(u.chBuf[:u.cursorPos])
	atCursorRune := " "
	if u.cursorPos < len(u.chBuf) {
		atCursorRune = string(u.chBuf[u.cursorPos])
	}
	afterCursor := ""
	if u.cursorPos+1 < len(u.chBuf) {
		afterCursor = string(u.chBuf[u.cursorPos+1:])
	}

	bgStyle := wig.Color("default")

	// Clear line
	view.SetContent(0, y, strings.Repeat(" ", vw), bgStyle)

	// Draw prefix and text before cursor starting at x = 0
	view.SetContent(0, y, promptPrefix+beforeCursor, bgStyle)

	// Draw cursor character (reversed) or the '^' for Ctrl-r mode
	cursorStyle := bgStyle.Reverse(true)
	if u.ctrlRMode {
		view.SetContent(len(beforeCursor)+1, y, "^", cursorStyle)
	} else {
		view.SetContent(len(beforeCursor)+1, y, atCursorRune, cursorStyle)
	}

	// Draw text after cursor
	if len(afterCursor) > 0 {
		view.SetContent(len(beforeCursor)+2, y, afterCursor, bgStyle)
	}

	// Show candidates visually on the lines above the prompt (like Vim)
	if len(u.candidates) > 1 {
		maxLen := 0
		for _, c := range u.candidates {
			if len([]rune(c)) > maxLen {
				maxLen = len([]rune(c))
			}
		}
		minColWidth := maxLen + 2
		if minColWidth < 10 {
			minColWidth = 10
		}

		cols := vw / minColWidth
		if cols == 0 {
			cols = 1
		}
		if cols > len(u.candidates) {
			cols = len(u.candidates)
		}
		colWidth := vw / cols

		rows := (len(u.candidates) + cols - 1) / cols
		for r := 0; r < rows; r++ {
			candY := y - 1 - r
			if candY < 0 {
				break
			}
			view.SetContent(0, candY, strings.Repeat(" ", vw), bgStyle)
		}

		for i, c := range u.candidates {
			r := i / cols
			col := i % cols
			candY := y - 1 - r
			if candY < 0 {
				break
			}
			x := col * colWidth
			itemStyle := bgStyle
			if i == u.candIdx {
				itemStyle = bgStyle.Reverse(true)
			}
			cWidth := colWidth
			if col == cols-1 && x+cWidth < vw {
				cWidth = vw - x
			}
			cStr := truncate(c, cWidth)
			view.SetContent(x, candY, cStr, itemStyle)
			if pad := cWidth - len([]rune(cStr)); pad > 0 {
				view.SetContent(x+len([]rune(cStr)), candY, strings.Repeat(" ", pad), itemStyle)
			}
		}
	}
}

func (u *uiCommandLine) Mode() wig.Mode {
	return wig.MODE_INSERT
}
