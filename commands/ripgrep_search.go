package commands

import (
	"fmt"
	"github.com/firstrow/wig"
	"github.com/firstrow/wig/rgcollect"
	"github.com/firstrow/wig/ui"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func CmdFindProjectFilePicker(ctx wig.Context) {
	rootDir, err := ctx.Editor.Projects.FindRoot(ctx.Buf)
	if err != nil {
		return
	}
	currentDir := rootDir
	isTreeView := ctx.Editor.Config.FilePickerView != "files"

	buildFlatItems := func() []ui.PickerItem[string] {
		cmd := exec.Command("rg", "--files")
		cmd.Dir = currentDir
		stdout, err := cmd.Output()
		if err != nil {
			ctx.Editor.LogError(err)
			return nil
		}
		items := []ui.PickerItem[string]{}
		for row := range strings.SplitSeq(string(stdout), "\n") {
			row = strings.TrimSpace(row)
			if len(row) == 0 {
				continue
			}
			fullPath := filepath.Join(currentDir, row)
			color := ""
			for _, b := range ctx.Editor.Buffers {
				if b.FilePath == fullPath && b.Dirty {
					color = "ui.popup.title"
					break
				}
			}
			items = append(items, ui.PickerItem[string]{
				Name:    row,
				Value:   row,
				FgColor: color,
			})
		}
		return items
	}

	buildTreeItems := func() []ui.PickerItem[string] {
		items := []ui.PickerItem[string]{}
		if currentDir != rootDir {
			items = append(items, ui.PickerItem[string]{
				Name:    "../",
				Value:   "..",
				FgColor: "ui.text.directory",
			})
		}
		entries, err := os.ReadDir(currentDir)
		if err != nil {
			ctx.Editor.LogError(err)
			return items
		}
		for _, entry := range entries {
			name := entry.Name()
			isDir := entry.IsDir()
			if isDir {
				name += "/"
			}
			value := name
			if isDir {
				value = strings.TrimSuffix(name, "/")
			}
			color := ""
			if isDir {
				color = "ui.text.directory"
			} else {
				fullPath := filepath.Join(currentDir, value)
				for _, b := range ctx.Editor.Buffers {
					if b.FilePath == fullPath && b.Dirty {
						color = "ui.popup.title"
						break
					}
				}
			}
			items = append(items, ui.PickerItem[string]{
				Name:    name,
				Value:   value,
				FgColor: color,
			})
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Value == ".." {
				return true
			}
			if items[j].Value == ".." {
				return false
			}
			isDirI := strings.HasSuffix(items[i].Name, "/")
			isDirJ := strings.HasSuffix(items[j].Name, "/")
			if isDirI != isDirJ {
				return isDirI
			}
			return items[i].Name < items[j].Name
		})
		return items
	}
	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		if i == nil {
			return
		}
		if isTreeView && strings.HasSuffix(i.Name, "/") {
			if i.Value == ".." {
				currentDir = filepath.Dir(currentDir)
			} else {
				currentDir = filepath.Join(currentDir, i.Value)
			}
			p.SetItems(buildTreeItems())
			p.ClearInput()
			return
		}

		defer ctx.Editor.PopUi()
		path := filepath.Join(currentDir, i.Value)
		ctx.Buf, err = ctx.Editor.OpenFile(path)
		if err != nil {
			return
		}
		ctx.Editor.ActiveWindow().VisitBuffer(ctx)
	}

	// Initialize picker with the correct initial items based on default view
	var initialItems []ui.PickerItem[string]
	if isTreeView {
		initialItems = buildTreeItems()
	} else {
		initialItems = buildFlatItems()
	}

	picker := ui.PickerInit(ctx.Editor, action, initialItems)

	if isTreeView {
		picker.SetTitle("Directory Tree (Tab: Files)")
	} else {
		picker.SetTitle("Find File (Tab: Tree)")
	}

	picker.OnKey("Tab", func(ctx wig.Context) {
		isTreeView = !isTreeView
		if isTreeView {
			picker.SetTitle("Directory Tree (Tab: Files)")
			picker.SetItems(buildTreeItems())
		} else {
			picker.SetTitle("Find File (Tab: Tree)")
			picker.SetItems(buildFlatItems())
		}
		picker.ClearInput()
		ctx.Editor.Redraw()
	})

	picker.OnKey("ctrl+o", func(ctx wig.Context) {
		CmdWindowVSplitLimited(ctx)
		wig.CmdWindowNext(ctx)
		picker.CallAction()
	})
}
func rgDoSearch(ctx wig.Context, pat string) {
	rootDir := ctx.Editor.Projects.GetRoot()

	// search with ripgrep using only the first word as the actual rg pattern.
	// everything else will be filtered with fuzzy matcher in ui/picker.
	searchFn := func(pat string) []ui.PickerItem[wig.Location] {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			return nil
		}

		parts := strings.Split(pat, " ")

		cmd := exec.Command("rg", "--json", "-S", parts[0])
		cmd.Dir = rootDir
		stdout, err := cmd.Output()
		if err != nil {
			ctx.Editor.LogError(err)
			return nil
		}

		locations := parseRgJSON(stdout)
		if len(locations) == 0 {
			return nil
		}

		items := make([]ui.PickerItem[wig.Location], 0, len(locations))
		for _, loc := range locations {
			relPath, _ := filepath.Rel(rootDir, loc.FilePath)
			if relPath == "" {
				relPath = loc.FilePath
			}
			fname := fmt.Sprintf("%s:%d %s", relPath, loc.Line, strings.TrimSpace(loc.Text))
			// Store the location with absolute path for opening
			absLoc := loc
			absLoc.FilePath = filepath.Join(rootDir, loc.FilePath)
			items = append(items, ui.PickerItem[wig.Location]{
				Name:     fname,
				Value:    absLoc,
				Location: absLoc,
			})
		}
		return items
	}

	action := func(p *ui.UiPicker[wig.Location], i *ui.PickerItem[wig.Location]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		buf, err := ctx.Editor.OpenFile(i.Value.FilePath)
		if err != nil {
			return
		}
		ctx.Buf = buf
		ctx.Editor.ActiveWindow().VisitBuffer(
			ctx,
			wig.Cursor{
				Line: max(i.Value.Line-1, 0),
				Char: i.Value.Char,
			},
		)
		wig.CmdCursorCenter(ctx)
	}

	p := ui.PickerInit(
		ctx.Editor,
		action,
		searchFn(pat),
	)

	p.SetInput(pat)
	p.SetTitle("Search Project (Ctrl-r) ...")

	p.OnChange(func() {
		p.SetItems(searchFn(p.GetInput()))
	})

	p.OnKey("ctrl+r", func(c wig.Context) {
		locs := p.GetFilteredLocations()
		ctx.Editor.PopUi()
		if len(locs) == 0 {
			ctx.Editor.EchoMessage("no search results to export")
			return
		}
		_ = rgcollect.SaveResults(locs)
		OpenLocationsInQuickfix(c, locs)
	})
}

func CmdSearchProject(ctx wig.Context) {
	rgDoSearch(ctx, "")
}

func CmdProjectSearchWordUnderCursor(ctx wig.Context) {
	word, ok := wig.WordOrSelectionUnderCursor(ctx)
	if !ok {
		ctx.Editor.EchoMessage("no word under cursor")
		return
	}

	rgDoSearch(ctx, word)
}
