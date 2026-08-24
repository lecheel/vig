package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

type TagEntry struct {
	Name string
	File string
	Addr string
	Kind string
	Line int
}

var tagsCache struct {
	path  string
	mtime time.Time
	tags  map[string][]TagEntry
}

func loadTags(rootDir string) map[string][]TagEntry {
	tagsPath := filepath.Join(rootDir, "tags")
	info, err := os.Stat(tagsPath)
	if err != nil {
		return nil
	}
	if tagsCache.path == tagsPath && tagsCache.mtime.Equal(info.ModTime()) {
		return tagsCache.tags
	}
	f, err := os.Open(tagsPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	tags := make(map[string][]TagEntry)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "!") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		file := parts[1]
		addr := parts[2]
		addr = strings.TrimSuffix(addr, ";\"")
		kind := ""
		lineNum := -1
		if n, err := strconv.Atoi(addr); err == nil {
			lineNum = n
		}
		for i := 3; i < len(parts); i++ {
			if !strings.Contains(parts[i], ":") {
				kind = parts[i]
				break
			}
		}
		tags[name] = append(tags[name], TagEntry{
			Name: name,
			File: file,
			Addr: addr,
			Kind: kind,
			Line: lineNum,
		})
	}
	tagsCache.path = tagsPath
	tagsCache.mtime = info.ModTime()
	tagsCache.tags = tags
	return tags
}

func updateTagsFile(rootDir string) error {
	if _, err := exec.LookPath("ctags"); err != nil {
		return fmt.Errorf("ctags not found")
	}
	tagsPath := filepath.Join(rootDir, "tags")
	cmd := exec.Command("ctags", "-R", "--fields=+K", "-f", tagsPath, rootDir)
	cmd.Dir = rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ctags failed: %v", err)
	}
	return nil
}

func findTagRoot(ctx wig.Context) string {
	rootDir, err := ctx.Editor.Projects.FindRoot(ctx.Buf)
	if err != nil || rootDir == "" {
		rootDir = ctx.Editor.Projects.GetRoot()
	}
	return rootDir
}

func jumpToTag(ctx wig.Context, e TagEntry, rootDir string) {
	filePath := e.File
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(rootDir, filePath)
	}
	nbuf, err := ctx.Editor.OpenFile(filePath)
	if err != nil {
		ctx.Editor.EchoMessage("tag: cannot open: " + err.Error())
		return
	}
	lineNum := e.Line
	if lineNum <= 0 {
		lineNum = findLineByPattern(nbuf, e.Addr)
	}
	cursor := wig.Cursor{Line: lineNum - 1, Char: 0}
	if cursor.Line < 0 {
		cursor.Line = 0
	}
	ctx.Buf = nbuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, cursor)
	wig.CmdCursorCenter(ctx.Editor.NewContext())
}

func findLineByPattern(buf *wig.Buffer, addr string) int {
	if n, err := strconv.Atoi(addr); err == nil {
		return n
	}
	if len(addr) < 2 {
		return 1
	}
	var pattern string
	if addr[0] == '/' {
		pattern = addr[1:]
		if strings.HasSuffix(pattern, "/") {
			pattern = pattern[:len(pattern)-1]
		}
	} else if addr[0] == '?' {
		pattern = addr[1:]
		if strings.HasSuffix(pattern, "?") {
			pattern = pattern[:len(pattern)-1]
		}
	} else {
		return 1
	}
	pattern = strings.TrimPrefix(pattern, "^")
	pattern = strings.TrimSuffix(pattern, "$")
	pattern = strings.ReplaceAll(pattern, "\\/", "/")
	pattern = strings.ReplaceAll(pattern, "\\\"", "\"")
	for i := 0; i < buf.Lines.Len; i++ {
		l := wig.CursorLineByNum(buf, i)
		if l != nil && strings.Contains(l.Value.String(), pattern) {
			return i + 1
		}
	}
	return 1
}

func CmdTagJump(ctx wig.Context) {
	word, ok := wig.WordOrSelectionUnderCursor(ctx)
	if !ok || word == "" {
		ctx.Editor.EchoMessage("tag: no word under cursor")
		return
	}
	jumpToTagName(ctx, word)
}

func CmdTag(ctx wig.Context) {
	word := strings.TrimSpace(ctx.Char)
	if word == "" {
		rootDir := findTagRoot(ctx)
		if err := updateTagsFile(rootDir); err != nil {
			ctx.Editor.EchoMessage("tag update: " + err.Error())
			return
		}
		tagsCache.tags = nil
		ctx.Editor.EchoMessage("tags updated")
		return
	}
	jumpToTagName(ctx, word)
}

func jumpToTagName(ctx wig.Context, word string) {
	rootDir := findTagRoot(ctx)
	tagsPath := filepath.Join(rootDir, "tags")
	if _, err := os.Stat(tagsPath); err != nil {
		if err := updateTagsFile(rootDir); err != nil {
			ctx.Editor.EchoMessage("tag: " + err.Error())
			return
		}
	}
	tags := loadTags(rootDir)
	if tags == nil {
		ctx.Editor.EchoMessage("tag: no tags file found")
		return
	}
	entries, ok := tags[word]
	if !ok || len(entries) == 0 {
		ctx.Editor.EchoMessage(fmt.Sprintf("tag: tag not found: %s", word))
		return
	}
	if len(entries) == 1 {
		jumpToTag(ctx, entries[0], rootDir)
		return
	}
	items := make([]ui.PickerItem[TagEntry], 0, len(entries))
	for _, e := range entries {
		display := fmt.Sprintf("%s %s:%d", e.Name, e.File, e.Line)
		if e.Kind != "" {
			display = fmt.Sprintf("[%s] %s %s:%d", e.Kind, e.Name, e.File, e.Line)
		}
		items = append(items, ui.PickerItem[TagEntry]{
			Name:  display,
			Value: e,
		})
	}
	action := func(p *ui.UiPicker[TagEntry], i *ui.PickerItem[TagEntry]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		jumpToTag(ctx, i.Value, rootDir)
	}
	picker := ui.PickerInit(ctx.Editor, action, items)
	picker.SetTitle(fmt.Sprintf("Tags: %s (%d)", word, len(entries)))
}
