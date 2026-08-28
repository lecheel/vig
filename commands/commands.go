package commands

import (
	"fmt"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/drivers/pipe"
	"github.com/firstrow/wig/ui"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func CmdThemeSelect(ctx wig.Context) {
	currentDir := ctx.Editor.RuntimeDir("themes")

	files, err := os.ReadDir(currentDir)
	if err != nil {
		ctx.Editor.LogError(err, true)
		return
	}

	themes := []string{}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".toml" {
			themes = append(themes, file.Name()[:len(file.Name())-5])
		}
	}

	items := make([]ui.PickerItem[string], 0, 256)
	for _, b := range themes {
		items = append(items, ui.PickerItem[string]{
			Name:   b,
			Value:  b,
			Active: false,
		})
	}

	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		wig.ApplyTheme(i.Value)
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		items,
	)
	picker.SetTitle("Themes")

	picker.OnSelect(func(item *ui.PickerItem[string]) {
		wig.ApplyTheme(item.Value)
		ctx.Editor.Redraw()
		ctx.Editor.ScreenSync()
	})
}

func CmdBufferPicker(ctx wig.Context) {
	items := make([]ui.PickerItem[*wig.Buffer], 0, 32)
	for _, b := range ctx.Editor.Buffers {
		items = append(items, ui.PickerItem[*wig.Buffer]{
			Name:   b.GetName(),
			Value:  b,
			Active: b == ctx.Editor.ActiveBuffer(),
		})
	}

	action := func(p *ui.UiPicker[*wig.Buffer], i *ui.PickerItem[*wig.Buffer]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		ctx.Buf = i.Value
		ctx.Editor.ActiveWindow().VisitBuffer(ctx)
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		items,
	)
	picker.SetTitle("Buffers")
	picker.OnKey("ctrl+o", func(ctx wig.Context) {
		wig.CmdWindowVSplit(ctx)
		wig.CmdWindowNext(ctx)
		picker.CallAction()
	})
}

func CmdGitFilesPicker(ctx wig.Context) {
	if !gitIsRepo() {
		ctx.Editor.EchoMessage("Not a git repository")
		return
	}

	posCache := wig.LoadPositionCache()
	showStatus := true

	buildStatusItems := func() []ui.PickerItem[string] {
		out := gitRun("status", "--porcelain=v2")
		lines := strings.Split(out, "\n")
		var items []ui.PickerItem[string]
		seen := make(map[string]bool)

		for _, line := range lines {
			if line == "" {
				continue
			}
			var code, path string
			switch line[0] {
			case '1', '2':
				parts := strings.SplitN(line, " ", 9)
				if len(parts) < 9 {
					continue
				}
				xy := parts[1]
				if len(xy) < 2 {
					continue
				}
				path = gitParsePorcelainPath(line[0], parts[8])
				x, y := xy[0], xy[1]
				switch {
				case x != '.' && y != '.':
					code = fmt.Sprintf("%c%c", x, y)
				case x != '.':
					code = fmt.Sprintf("%c ", x)
				default:
					code = fmt.Sprintf(" %c", y)
				}
			case '?':
				path = gitUnquotePath(strings.TrimPrefix(line, "? "))
				code = "??"
			default:
				continue
			}

			if path != "" && !seen[path] {
				seen[path] = true
				items = append(items, ui.PickerItem[string]{
					Name:  fmt.Sprintf("[%s] %s", code, path),
					Value: path,
				})
			}
		}
		return items
	}

	buildLastCommitItems := func() []ui.PickerItem[string] {
		out := gitRun("diff-tree", "--no-commit-id", "--name-status", "-r", "HEAD")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		var items []ui.PickerItem[string]
		for _, l := range lines {
			if l == "" {
				continue
			}
			parts := strings.SplitN(l, "\t", 2)
			if len(parts) < 2 {
				continue
			}
			code := parts[0]
			path := gitUnquotePath(parts[1])
			items = append(items, ui.PickerItem[string]{
				Name:  fmt.Sprintf("[%s] %s", code, path),
				Value: path,
			})
		}
		return items
	}

	buildItems := func() []ui.PickerItem[string] {
		if showStatus {
			return buildStatusItems()
		}
		return buildLastCommitItems()
	}

	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}

		filePath := i.Value
		if !filepath.IsAbs(filePath) {
			rootDir := ctx.Editor.Projects.GetRoot()
			filePath = filepath.Join(rootDir, filePath)
		}

		var targetBuf *wig.Buffer
		for _, b := range ctx.Editor.Buffers {
			if b.FilePath == filePath {
				targetBuf = b
				break
			}
		}

		if targetBuf != nil {
			targetBuf.OpenCount++
			ctx.Buf = targetBuf
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
		} else {
			buf, err := ctx.Editor.OpenFile(filePath)
			if err != nil {
				ctx.Editor.EchoMessage("Cannot open: " + err.Error())
				return
			}
			buf.OpenCount = posCache.Files[filePath].OpenCount + 1
			ctx.Buf = buf

			targetLine := posCache.Files[filePath].Line
			if targetLine >= buf.Lines.Len {
				targetLine = buf.Lines.Len - 1
			}
			ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: targetLine, Char: 0})
			wig.CmdCursorCenter(ctx)
		}
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		buildItems(),
	)

	updateTitle := func() {
		if showStatus {
			picker.SetTitle("Git Files (Status) [Ins: Last Commit]")
		} else {
			hash := strings.TrimSpace(gitRun("rev-parse", "--short", "HEAD"))
			if hash != "" {
				picker.SetTitle(fmt.Sprintf("Git Files (Commit: %s) [Ins: Status]", hash))
			} else {
				picker.SetTitle("Git Files (Last Commit) [Ins: Status]")
			}
		}
	}
	updateTitle()

	picker.OnKey("Insert", func(ctx wig.Context) {
		showStatus = !showStatus
		updateTitle()
		picker.SetItems(buildItems())
		ctx.Editor.Redraw()
	})

	picker.OnKey("ctrl+o", func(ctx wig.Context) {
		wig.CmdWindowVSplit(ctx)
		wig.CmdWindowNext(ctx)
		picker.CallAction()
	})
}

func CmdMRUBufferPicker(ctx wig.Context) {
	posCache := wig.LoadPositionCache()
	sortByRecent := true

	type kv struct {
		Key string
		Val wig.PositionEntry
	}

	buildItems := func() []ui.PickerItem[string] {
		var entries []kv
		for k, v := range posCache.Files {
			if _, err := os.Stat(k); err == nil {
				entries = append(entries, kv{k, v})
			}
		}

		sort.Slice(entries, func(i, j int) bool {
			if sortByRecent {
				return entries[i].Val.Timestamp > entries[j].Val.Timestamp
			}
			return entries[i].Val.OpenCount > entries[j].Val.OpenCount
		})

		res := make([]ui.PickerItem[string], 0, len(entries))
		for _, e := range entries {
			var prefix string
			if sortByRecent {
				if e.Val.Timestamp > 0 {
					prefix = time.Unix(e.Val.Timestamp, 0).Format("01-02 15:04")
				} else {
					prefix = "--/-- --:--"
				}
			} else {
				prefix = fmt.Sprintf("%11d", e.Val.OpenCount)
			}
			res = append(res, ui.PickerItem[string]{
				Name:  fmt.Sprintf("[%s] %s", prefix, e.Key),
				Value: e.Key,
			})
		}
		return res
	}

	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}

		filePath := i.Value
		var targetBuf *wig.Buffer
		for _, b := range ctx.Editor.Buffers {
			if b.FilePath == filePath {
				targetBuf = b
				break
			}
		}

		if targetBuf != nil {
			targetBuf.OpenCount++
			ctx.Buf = targetBuf
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
		} else {
			buf, err := ctx.Editor.OpenFile(filePath)
			if err != nil {
				return
			}
			buf.OpenCount = posCache.Files[filePath].OpenCount + 1
			ctx.Buf = buf

			targetLine := posCache.Files[filePath].Line
			if targetLine >= buf.Lines.Len {
				targetLine = buf.Lines.Len - 1
			}
			ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: targetLine, Char: 0})
			wig.CmdCursorCenter(ctx)
		}
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		buildItems(),
	)

	updateTitle := func() {
		if sortByRecent {
			picker.SetTitle("MRU Buffers (Most Recent) [Ins: Most Used]")
		} else {
			picker.SetTitle("MRU Buffers (Most Used) [Ins: Most Recent]")
		}
	}
	updateTitle()

	picker.OnKey("Insert", func(ctx wig.Context) {
		sortByRecent = !sortByRecent
		updateTitle()
		picker.SetItems(buildItems())
		ctx.Editor.Redraw()
	})

	picker.OnKey("Delete", func(ctx wig.Context) {
		item := picker.GetActiveItem()
		if item == nil {
			return
		}
		filePath := item.Value

		delete(posCache.Files, filePath)
		posCache.Save()

		picker.SetItems(buildItems())
		ctx.Editor.Redraw()
	})
}

func CmdCommandPalettePicker(ctx wig.Context) {
	items := make([]ui.PickerItem[wig.CmdDefinition], 0, 128)

	for k, v := range wig.AllCommands {
		name := fmt.Sprintf("%s [%s]", v.Desc, k)
		items = append(items, ui.PickerItem[wig.CmdDefinition]{
			Name:  name,
			Value: v,
		})
	}

	action := func(p *ui.UiPicker[wig.CmdDefinition], i *ui.PickerItem[wig.CmdDefinition]) {
		ctx.Editor.PopUi()

		if i == nil {
			return
		}

		switch cmd := i.Value.Fn.(type) {
		case func(wig.Context):
			cmd(ctx)
		}
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		items,
	)
	picker.SetTitle("Command Palette")
}

func CmdExecute(ctx wig.Context) {
	if ctx.Buf.Driver == nil {
		ctx.Buf.Driver = pipe.New(ctx.Editor)
	}
	cur := wig.ContextCursorGet(ctx)
	ctx.Buf.Driver.Exec(ctx.Editor, ctx.Buf, wig.CursorLine(ctx.Buf, cur))
}

func CmdCurrentBufferDirFilePicker(ctx wig.Context) {
	rootDir := ctx.Editor.Projects.Dir(ctx.Buf)
	ctx.Editor.EchoMessage("listing dir: " + rootDir)

	getItems := func(dir string) []ui.PickerItem[string] {
		cmd := exec.Command("ls", "-ap")
		cmd.Dir = dir
		stdout, err := cmd.Output()
		if err != nil {
			ctx.Editor.LogMessage(string(stdout))
			ctx.Editor.LogError(err)
			return nil
		}

		items := []ui.PickerItem[string]{}

		for row := range strings.SplitSeq(string(stdout), "\n") {
			row = strings.TrimSpace(row)
			if len(row) == 0 {
				continue
			}
			if row == "./" {
				continue
			}

			items = append(items, ui.PickerItem[string]{
				Name:  row,
				Value: row,
			})
		}
		return items
	}

	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		// create new file
		if i == nil {
			fp := path.Join(rootDir, p.GetInput())
			buf, err := ctx.Editor.OpenFile(fp)
			if err != nil {
				buf = wig.EditorInst.BufferFindByFilePath(fp, true)
			}
			ctx.Buf = buf
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
			ctx.Editor.PopUi()
			return
		}

		// list directory
		if strings.HasSuffix(i.Name, "/") {
			fp := path.Join(rootDir, i.Value)
			ctx.Editor.EchoMessage("listing dir: " + fp)
			rootDir = fp
			p.SetItems(getItems(rootDir))
			p.ClearInput()
			return
		}

		buf, err := ctx.Editor.OpenFile(rootDir + "/" + i.Value)
		if err != nil {
			return
		}
		ctx.Buf = buf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx)
		ctx.Editor.PopUi()
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		getItems(rootDir),
	)
	picker.SetTitle("Files")
}

func CmdFormatBuffer(ctx wig.Context) {
	if strings.HasSuffix(ctx.Buf.FilePath, ".go") {
		var cmd *exec.Cmd
		if _, err := exec.LookPath("goimports"); err == nil {
			cmd = exec.Command("goimports", "-w", ctx.Buf.FilePath)
		} else {
			cmd = exec.Command("go", "fmt", ctx.Buf.FilePath)
		}
		stdout, err := cmd.CombinedOutput()
		if err != nil {
			ctx.Editor.LogError(err)
			ctx.Editor.LogMessage(string(stdout))
			return
		}
		reloadBufferPostFormat(ctx)
	}

	if strings.HasSuffix(ctx.Buf.FilePath, ".odin") {
		formatcmd := fmt.Sprintf("odinfmt %s -w", ctx.Buf.FilePath)
		cmd := exec.Command("bash", "-c", formatcmd)
		stdout, err := cmd.Output()
		if err != nil {
			ctx.Editor.LogMessage(err.Error())
			ctx.Editor.LogMessage(string(stdout))
			return
		}
		reloadBufferPostFormat(ctx)
	}

	if strings.HasSuffix(ctx.Buf.FilePath, ".rs") {
		cmd := exec.Command("rustfmt", ctx.Buf.FilePath)
		stdout, err := cmd.CombinedOutput()
		if err != nil {
			ctx.Editor.LogError(err)
			ctx.Editor.LogMessage(string(stdout))
			return
		}
		reloadBufferPostFormat(ctx)
	}
}

// reloadBufferPostFormat reloads the buffer after formatting. If FormatOnSave
// is enabled, it uses a transactional reload so undo/redo history is preserved.
// Otherwise, it falls back to a standard hard reload.
func reloadBufferPostFormat(ctx wig.Context) {
	if ctx.Editor.Config.FormatOnSave {
		data, err := os.ReadFile(ctx.Buf.FilePath)
		if err != nil {
			ctx.Editor.EchoMessage(err.Error())
			return
		}
		wig.ReloadBufferContent(ctx, string(data))
		if ctx.Buf.Highlighter != nil {
			ctx.Buf.Highlighter.Build()
		}
		ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: ctx.Buf})
	} else {
		CmdReloadBuffer(ctx)
	}
}

func CmdSearchWordUnderCursor(ctx wig.Context) {
	word, _ := wig.WordOrSelectionUnderCursor(ctx)
	wig.LastSearchPattern = word
	wig.SearchNext(ctx, word)
}

func CmdFormatBufferAndSave(ctx wig.Context) {
	// Temporarily disable FormatOnSave to avoid double formatting inside CmdSaveFile
	origFormatOnSave := ctx.Editor.Config.FormatOnSave
	ctx.Editor.Config.FormatOnSave = false
	// This first save ensures the formatter has the latest content from the buffer.
	wig.CmdSaveFile(ctx)
	ctx.Editor.Config.FormatOnSave = origFormatOnSave

	CmdFormatBuffer(ctx) // This formats the content and writes it to disk, also reloads buffer transactionally.
	// After formatting and reloading, we call the save function again to ensure
	// the buffer's dirty state is correctly reset and SavedAtPosition is updated,
	// and to provide feedback for the 'save' part of 'format and save'.
	CmdSaveFileWithFeedback(ctx)

	ctx.Editor.Lsp.DidClose(ctx.Buf)
	ctx.Editor.Lsp.DidOpen(ctx.Buf)
	if ctx.Buf.Highlighter != nil {
		ctx.Buf.Highlighter.Build()
	}
}

// CmdToggleBool toggles boolean and binary values under the cursor (true/false, True/False, TRUE/FALSE, etc.)
func CmdToggleBool(ctx wig.Context) {
	if ctx.Buf == nil {
		return
	}

	cur := wig.ContextCursorGet(ctx)
	if cur == nil {
		return
	}

	line := wig.CursorLine(ctx.Buf, cur)
	if line == nil || len(line.Value) == 0 {
		return
	}

	chars := line.Value
	idx := cur.Char
	if idx >= len(chars) {
		idx = len(chars) - 1
	}

	isWordChar := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
	}

	if !isWordChar(chars[idx]) {
		if idx > 0 && isWordChar(chars[idx-1]) {
			idx--
		} else {
			return
		}
	}

	start := idx
	for start > 0 && isWordChar(chars[start-1]) {
		start--
	}

	end := idx
	for end < len(chars) && isWordChar(chars[end]) {
		end++
	}

	if start >= end {
		return
	}

	word := string(chars[start:end])
	var newWord string
	switch word {
	case "true":
		newWord = "false"
	case "false":
		newWord = "true"
	case "True":
		newWord = "False"
	case "False":
		newWord = "True"
	case "TRUE":
		newWord = "FALSE"
	case "FALSE":
		newWord = "TRUE"
	case "yes":
		newWord = "no"
	case "no":
		newWord = "yes"
	case "Yes":
		newWord = "No"
	case "No":
		newWord = "Yes"
	case "YES":
		newWord = "NO"
	case "NO":
		newWord = "YES"
	case "on":
		newWord = "off"
	case "off":
		newWord = "on"
	case "On":
		newWord = "Off"
	case "Off":
		newWord = "On"
	case "ON":
		newWord = "OFF"
	case "OFF":
		newWord = "ON"
	case "1":
		newWord = "0"
	case "0":
		newWord = "1"
	default:
		return
	}

	if ctx.Buf.TxStart() {
		defer ctx.Buf.TxEnd()
	}

	wig.TextDelete(ctx.Buf, &wig.Selection{
		Start: wig.Cursor{Line: cur.Line, Char: start},
		End:   wig.Cursor{Line: cur.Line, Char: end},
	})
	lineNode := wig.CursorLineByNum(ctx.Buf, cur.Line)
	if lineNode != nil {
		wig.TextInsert(ctx.Buf, lineNode, start, newWord)
	}

	if cur.Char > start+len(newWord) {
		cur.Char = start + len(newWord)
	}
	ctx.Editor.Redraw()
}

// CmdOpenConfig opens ~/.config/wig/config.toml for editing, creating it if it doesn't exist.
func CmdOpenConfig(ctx wig.Context) {
	home, err := os.UserHomeDir()
	if err != nil {
		ctx.Editor.EchoMessage("Cannot find home directory: " + err.Error())
		return
	}
	configDir := filepath.Join(home, ".config", "wig")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.WriteFile(configPath, []byte{}, 0644)
	}

	buf, err := ctx.Editor.OpenFile(configPath)
	if err != nil {
		ctx.Editor.EchoMessage("Cannot open config file: " + err.Error())
		return
	}
	ctx.Buf = buf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx)
}

// NotifyOnSave controls whether a visual toast popup notification is shown on successful save (default: false).
// Configured via notify_on_save in ~/.config/wig/config.toml under [editor].
var NotifyOnSave = false

// CmdSaveFileWithFeedback saves the current buffer to disk and displays status bar & notification feedback.
// Handles both :w and :w <filename> for unnamed [No Name] buffers.
func CmdSaveFileWithFeedback(ctx wig.Context) {
	if ctx.Buf == nil {
		return
	}

	// If a filename argument was provided (e.g. :w newfile.txt)
	if ctx.Char != "" {
		filePath := ctx.Char
		if !filepath.IsAbs(filePath) {
			rootDir := ctx.Editor.Projects.GetRoot()
			if rootDir != "" {
				filePath = filepath.Join(rootDir, filePath)
			} else {
				if cwd, err := os.Getwd(); err == nil {
					filePath = filepath.Join(cwd, filePath)
				}
			}
		}
		ctx.Buf.FilePath = filePath
	}

	if ctx.Buf.FilePath == "" || strings.HasPrefix(ctx.Buf.FilePath, "[") {
		ctx.Editor.EchoMessage("No file name")
		return
	}

	if err := ctx.Buf.Save(); err != nil {
		ctx.Editor.EchoMessage("Save error: " + err.Error())
		return
	}

	ctx.Buf.Dirty = false
	wig.CmdSaveFile(ctx)

	lineCount := ctx.Buf.CountLines()
	var byteCount int64
	if fi, err := os.Stat(ctx.Buf.FilePath); err == nil {
		byteCount = fi.Size()
	} else {
		byteCount = int64(len(ctx.Buf.String()))
	}

	msg := fmt.Sprintf("\"%s\" %dL, %dB written", ctx.Buf.GetName(), lineCount, byteCount)
	ctx.Editor.EchoMessage(msg)
	if NotifyOnSave {
		ui.Notify(msg, ui.NotifySuccess)
	}
}

func CmdMakeBuild(ctx wig.Context) {
	CmdFormatBufferAndSave(ctx)
	cmd := exec.Command("make", "run")
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		ctx.Editor.LogMessage(err.Error())
		ctx.Editor.LogMessage(string(stdout))
		mbuf := ctx.Editor.BufferFindByFilePath("[Messages]", true)
		ctx.Editor.EnsureBufferIsVisible(mbuf)
		return
	}
	ctx.Editor.EchoMessage("[build ok]")
}

func CmdMakeTest(ctx wig.Context) {
	cmd := exec.Command("make", "test")
	stdout, err := cmd.CombinedOutput()
	std := string(stdout)
	mbuf := ctx.Editor.BufferFindByFilePath("[make test]", true)
	if err != nil || strings.Contains(std, "leak") {

		mbuf.TxStart()
		mbuf.ResetLines()
		mbuf.Append(std)
		mbuf.TxEnd()

		ctx.Editor.EnsureBufferIsVisible(mbuf)
		return
	}

	mbuf.TxStart()
	mbuf.ResetLines()
	mbuf.Append(std)
	mbuf.TxEnd()

	ctx.Editor.EchoMessage("[tests ok]")
}

func CmdSearchLine(ctx wig.Context) {
	items := make([]ui.PickerItem[int], 0, 256)

	line := ctx.Buf.Lines.First()
	i := 0
	for line != nil {
		items = append(items, ui.PickerItem[int]{
			Name:   line.Value.String(),
			Value:  i,
			Active: false,
		})

		i++
		line = line.Next()
	}

	action := func(p *ui.UiPicker[int], i *ui.PickerItem[int]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
			Line: i.Value,
			Char: 0,
		})
		wig.CmdCursorBeginningOfTheLine(ctx)
		wig.CmdCursorCenter(ctx)
	}

	picker := ui.PickerInit(
		ctx.Editor,
		action,
		items,
	)
	picker.SetTitle("Search Line")
}

func CmdGotoDefinition(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	ctx.Editor.EchoMessage(fmt.Sprintf("gd: calling LSP definition at %d:%d", cur.Line, cur.Char))
	filePath, cursor := ctx.Editor.Lsp.Definition(ctx.Buf, *cur)
	if filePath == "" {
		ctx.Editor.EchoMessage("gd: LSP returned no definition (not started, no result, or error)")
		return
	}

	ctx.Editor.EchoMessage(fmt.Sprintf("gd: jumping to %s:%d:%d", filePath, cursor.Line, cursor.Char))

	nbuf, err := ctx.Editor.OpenFile(filePath)
	if err != nil {
		ctx.Editor.EchoMessage("gd: open file error: " + err.Error())
		return
	}

	ctx.Buf = nbuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, cursor)
	wig.CmdCursorCenter(ctx.Editor.NewContext())
}

func CmdGotoDefinitionOtherWindow(ctx wig.Context) {
	CmdViewDefinitionOtherWindow(ctx)
	wig.CmdWindowNext(ctx)
}

func CmdViewDefinitionOtherWindow(ctx wig.Context) {
	curWin := ctx.Editor.ActiveWindow()
	cur := wig.ContextCursorGet(ctx)

	if len(ctx.Editor.Windows()) == 1 {
		wig.CmdWindowVSplit(ctx)
	}

	wig.CmdWindowNext(ctx)
	ctx.Win = nil

	ctx.Editor.ActiveWindow().VisitBuffer(ctx, *cur)
	CmdGotoDefinition(ctx)
	ctx.Editor.SetActiveWindow(curWin)
}

// CmdWorkspaceListPicker opens a fuzzy picker listing every workspace
// alongside the files it contains. Selecting a workspace captures the
// current workspace state into the persistence cache, switches to the
// target workspace, and restores its files from cache if it is empty.
//
// The picker also supports pressing Delete to clear a workspace's
// cached entry.
func CmdWorkspaceListPicker(ctx wig.Context) {
	cache := wig.LoadWorkspaceCache()
	cache.CaptureAll(ctx.Editor)

	buildItems := func() []ui.PickerItem[int] {
		items := make([]ui.PickerItem[int], 0, len(ctx.Editor.Workspaces))
		for i := 0; i < len(ctx.Editor.Workspaces); i++ {
			var label string
			if i == ctx.Editor.ActiveWorkspace {
				label = fmt.Sprintf("ws%d *", i)
			} else {
				label = fmt.Sprintf("ws%d", i)
			}

			entry := cache.Workspaces[i]
			var filesStr string
			if len(entry.Files) == 0 {
				filesStr = "(empty)"
			} else {
				names := make([]string, 0, len(entry.Files))
				for _, f := range entry.Files {
					names = append(names, filepath.Base(f))
				}
				filesStr = strings.Join(names, " ")
			}

			items = append(items, ui.PickerItem[int]{
				Name:   fmt.Sprintf("%s: %s", label, filesStr),
				Value:  i,
				Active: i == ctx.Editor.ActiveWorkspace,
			})
		}
		return items
	}

	action := func(p *ui.UiPicker[int], i *ui.PickerItem[int]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}

		target := i.Value
		if target == ctx.Editor.ActiveWorkspace {
			return
		}

		// Capture current workspace before switching
		cache.CaptureWorkspace(ctx.Editor.ActiveWorkspace, ctx.Editor.GetActiveWorkspace())
		cache.Save()

		// Ensure target workspace has at least one window
		ws := ctx.Editor.GetWorkspace(target)
		if len(ws.Windows) == 0 {
			win := wig.CreateWindow(nil)
			ws.Windows = []*wig.Window{win}
			ws.Num = target
			ws.ActiveWindow = win
		}

		ctx.Editor.ActiveWorkspace = target

		// Restore files from cache if workspace is empty
		cache.RestoreWorkspace(ctx.Editor, target)
		ctx.Editor.Redraw()
	}

	picker := ui.PickerInit(ctx.Editor, action, buildItems())
	picker.SetTitle("Workspaces [Enter: Switch  Del: Clear cache]")

	picker.OnKey("Delete", func(ctx wig.Context) {
		item := picker.GetActiveItem()
		if item == nil {
			return
		}
		delete(cache.Workspaces, item.Value)
		cache.Save()
		picker.SetItems(buildItems())
		ctx.Editor.Redraw()
	})
}

func CmdLspShowSignature(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	sign := ctx.Editor.Lsp.Signature(ctx.Buf, *cur)
	if sign != "" {
		ctx.Editor.EchoMessage(sign)
	}
}

func CmdLspHover(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	sign := ctx.Editor.Lsp.Hover(ctx.Buf, *cur)
	if sign != "" {
		ui.HoverInit(ctx, sign)
	}
}

func CmdLspShowDiagnostics(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	diagnostics := ctx.Editor.Lsp.Diagnostics(ctx.Buf, cur.Line)
	if len(diagnostics) == 0 {
		return
	}

	for _, info := range diagnostics {
		if cur.Char >= int(info.Range.Start.Character) && cur.Char <= int(info.Range.End.Character) {
			ctx.Editor.EchoMessage(info.Message)
			return
		}
	}
}

func CmdReloadBuffer(ctx wig.Context) {
	err := wig.BufferReloadFile(ctx.Buf)
	if err != nil {
		ctx.Editor.EchoMessage(err.Error())
	}
	ctx.Buf.Highlighter.Build()
	ctx.Editor.Events.Broadcast(wig.EventBufferReloaded{Buf: ctx.Buf})
}

// CmdLspStatus shows current LSP connection state and diagnostics for the
// active buffer's project root. Useful for debugging "gd not working".
func CmdLspStatus(ctx wig.Context) {
	buf := ctx.Buf
	if buf == nil {
		ctx.Editor.EchoMessage("no active buffer")
		return
	}
	root, _ := ctx.Editor.Projects.FindRoot(buf)
	ext := filepath.Ext(buf.FilePath)
	conf, confFound := wig.LspConfigByFileName(buf.FilePath)
	diags := ctx.Editor.Lsp.AllDiagnostics(buf)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n", buf.FilePath))
	sb.WriteString(fmt.Sprintf("Ext: %s\n", ext))
	sb.WriteString(fmt.Sprintf("Root: %s\n", root))
	sb.WriteString(fmt.Sprintf("LSP enabled (config): %v\n", ctx.Editor.Config.LspEnabled))
	sb.WriteString(fmt.Sprintf("Language config found: %v", confFound))
	if confFound {
		sb.WriteString(fmt.Sprintf(" (cmd=%s args=%v)", conf.Command, conf.Args))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Diagnostics: %d\n", len(diags)))
	for i, d := range diags {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("  ... +%d more\n", len(diags)-5))
			break
		}
		msg := strings.ReplaceAll(d.Message, "\n", " ")
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("  L%d:C%d [%s] %s\n",
			d.Range.Start.Line+1, d.Range.Start.Character+1, d.Severity, msg))
	}
	// Print to messages buffer AND echo the first line
	ctx.Editor.LogMessage(sb.String())
	ctx.Editor.EchoMessage(fmt.Sprintf("LSP: root=%s ext=%s confFound=%v enabled=%v diags=%d (see :messages for details)",
		root, ext, confFound, ctx.Editor.Config.LspEnabled, len(diags)))
}

// CmdInfo sends a notification to the top-right notification area.
// If an argument/message is provided (e.g. :info hello), it displays that text.
// Otherwise, it displays current buffer and cursor position info.
func CmdInfo(ctx wig.Context) {
	if ctx.Char != "" {
		ui.Notify(ctx.Char, ui.NotifyInfo)
		return
	}

	if ctx.Buf == nil {
		ui.Notify("No active buffer", ui.NotifyWarn)
		return
	}

	cur := wig.ContextCursorGet(ctx)
	totalLines := ctx.Buf.CountLines()
	curLine := 1
	curChar := 1
	if cur != nil {
		curLine = cur.Line + 1
		curChar = cur.Char + 1
	}

	percent := 0
	if totalLines > 0 {
		percent = (curLine * 100) / totalLines
	}

	dirtyStr := ""
	if ctx.Buf.Dirty {
		dirtyStr = " [+]"
	}

	msg := fmt.Sprintf("\"%s\"%s line %d of %d --%d%%-- col %d",
		ctx.Buf.GetName(),
		dirtyStr,
		curLine,
		totalLines,
		percent,
		curChar,
	)
	ui.Notify(msg, ui.NotifyInfo)
	ctx.Editor.EchoMessage(msg)
}
