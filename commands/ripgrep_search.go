package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/rgcollect"
	"github.com/firstrow/wig/ui"
)

func CmdFindProjectFilePicker(ctx wig.Context) {
	rootDir, err := ctx.Editor.Projects.FindRoot(ctx.Buf)
	if err != nil {
		return
	}

	cmd := exec.Command("rg", "--files")
	cmd.Dir = rootDir
	stdout, err := cmd.Output()
	if err != nil {
		ctx.Editor.LogError(err)
		return
	}

	items := []ui.PickerItem[string]{}

	for row := range strings.SplitSeq(string(stdout), "\n") {
		row = strings.TrimSpace(row)
		if len(row) == 0 {
			continue
		}
		items = append(items, ui.PickerItem[string]{
			Name:  row,
			Value: row,
		})
	}

	picker := ui.PickerInit(
		ctx.Editor,
		func(_ *ui.UiPicker[string], i *ui.PickerItem[string]) {
			defer ctx.Editor.PopUi()
			if i == nil {
				return
			}
			path := rootDir + "/" + i.Value
			ctx.Buf, err = ctx.Editor.OpenFile(path)
			if err != nil {
				return
			}
			ctx.Editor.ActiveWindow().VisitBuffer(ctx)
		},
		items,
	)
	picker.SetTitle("Find File")

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
