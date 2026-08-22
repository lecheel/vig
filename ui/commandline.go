package ui

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

var cmdHistory []string

type uiCommandLine struct {
	e          *wig.Editor
	keymap     *wig.KeyHandler
	chBuf      []rune
	cursorPos  int
	historyIdx int
	candidates []string
	candIdx    int
	ctrlRMode  bool
}

func (u *uiCommandLine) Plane() wig.RenderPlane {
	return wig.PlaneEditor
}

func CmdLineInit(ctx wig.Context) {
	u := &uiCommandLine{
		e:          ctx.Editor,
		chBuf:      make([]rune, 0, 32),
		cursorPos:  0,
		historyIdx: len(cmdHistory), // Start at the end of history
		candidates: []string{},
		candIdx:    -1,
	}

	// Pre-fill range for visual modes, exactly like Vim
	if ctx.Buf != nil && (ctx.Buf.Mode() == wig.MODE_VISUAL || ctx.Buf.Mode() == wig.MODE_VISUAL_LINE || ctx.Buf.Mode() == wig.MODE_VISUAL_BLOCK) {
		u.chBuf = []rune("'<,'>")
	}
	u.keymap = wig.NewKeyHandler(wig.ModeKeyMap{
		wig.MODE_INSERT: wig.KeyMap{
			"Esc": func(ctx wig.Context) {
				ctx.Editor.PopUi()
			},
			"Enter": func(ctx wig.Context) {
				cmd := string(u.chBuf)
				if strings.TrimSpace(cmd) != "" {
					cmdHistory = append(cmdHistory, cmd)
				}
				u.execute(cmd)
			},
			"Tab": func(ctx wig.Context) {
				u.autocomplete()
			},
			"Up": func(ctx wig.Context) {
				if u.historyIdx > 0 {
					u.historyIdx--
					u.chBuf = []rune(cmdHistory[u.historyIdx])
					u.candidates = []string{}
					u.candIdx = -1
				}
			},
			"Down": func(ctx wig.Context) {
				if u.historyIdx < len(cmdHistory)-1 {
					u.historyIdx++
					u.chBuf = []rune(cmdHistory[u.historyIdx])
					u.candidates = []string{}
					u.candIdx = -1
				} else {
					u.historyIdx = len(cmdHistory)
					u.chBuf = []rune{}
					u.candidates = []string{}
					u.candIdx = -1
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
			// Grab the full contiguous non-whitespace block under cursor from buffer
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

					// Find start of word (move left if on space, then find word boundary)
					start := idx
					for start > 0 && unicode.IsSpace(chars[start]) {
						start--
					}
					for start > 0 && !unicode.IsSpace(chars[start-1]) {
						start--
					}

					// Find end of word
					end := start
					for end < len(chars)-1 && !unicode.IsSpace(chars[end]) {
						end++
					}

					if end >= start && !unicode.IsSpace(chars[start]) {
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
		case tcell.KeyCtrlK:
			u.chBuf = u.chBuf[:u.cursorPos]
			u.candidates = []string{}
			u.candIdx = -1
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
		}
		return
	case tcell.KeyDelete:
		if u.cursorPos < len(u.chBuf) {
			u.chBuf = append(u.chBuf[:u.cursorPos], u.chBuf[u.cursorPos+1:]...)
			u.candidates = []string{}
			u.candIdx = -1
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
		u.chBuf = append(u.chBuf, 0) // dummy to expand slice
		copy(u.chBuf[u.cursorPos+1:], u.chBuf[u.cursorPos:])
		u.chBuf[u.cursorPos] = ev.Rune()
		u.cursorPos++
		u.candidates = []string{}
		u.candIdx = -1
		return
	}
}

func (u *uiCommandLine) autocomplete() {
	input := string(u.chBuf)
	parts := strings.SplitN(input, " ", 2)

	// File path completion for :e or :edit
	if len(parts) == 2 && (parts[0] == "e" || parts[0] == "edit") {
		cmdPart := parts[0]
		prefix := parts[1]

		if len(u.candidates) > 0 {
			u.candIdx++
			if u.candIdx >= len(u.candidates) {
				u.candIdx = 0
			}
			u.chBuf = []rune(fmt.Sprintf("%s %s", cmdPart, u.candidates[u.candIdx]))
			u.cursorPos = len(u.chBuf)
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
		u.candIdx++
		if u.candIdx >= len(u.candidates) {
			u.candIdx = 0
		}
		u.chBuf = []rune(u.candidates[u.candIdx])
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
		replaceCmd := restCmd[2:] // "pat/rep/flags"
		parts := strings.SplitN(replaceCmd, "/", 3)
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
			u.e.EchoMessage("Invalid regex: " + err.Error())
			return
		}

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
	view.SetContent(0, vh-1, strings.Repeat(" ", vw), bgStyle)

	// Draw prefix and text before cursor
	view.SetContent(0, vh-1, promptPrefix+beforeCursor, bgStyle)

	// Draw cursor character (reversed) or the '^' for Ctrl-r mode
	cursorStyle := bgStyle.Reverse(true)
	if u.ctrlRMode {
		view.SetContent(len(beforeCursor)+1, vh-1, "^", cursorStyle)
	} else {
		view.SetContent(len(beforeCursor)+1, vh-1, atCursorRune, cursorStyle)
	}

	// Draw text after cursor
	if len(afterCursor) > 0 {
		view.SetContent(len(beforeCursor)+2, vh-1, afterCursor, bgStyle)
	}

	// Show candidates visually on the line above the prompt (like Vim)
	if len(u.candidates) > 1 {
		candStr := strings.Join(u.candidates, "  ")
		if len(candStr) > vw {
			candStr = candStr[:vw]
		}
		if vh-2 >= 0 {
			view.SetContent(0, vh-2, candStr, bgStyle)
		}
	}
}

func (u *uiCommandLine) Mode() wig.Mode {
	return wig.MODE_INSERT
}
