package commands

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/rgcollect"
)

// rgSearchLocations runs `rg --json -S <word>` in the project root and
// returns the parsed results as a slice of wig.Location.
func rgSearchLocations(ctx wig.Context, word string) []wig.Location {
	rootDir := ctx.Editor.Projects.GetRoot()

	cmd := exec.Command("rg", "--json", "-S", word)
	cmd.Dir = rootDir
	stdout, err := cmd.Output()
	if err != nil {
		// rg exits with 1 when no matches found, 2 on error
		return nil
	}

	type rgMatch struct {
		Type string `json:"type"`
		Data struct {
			Path struct {
				Text string `json:"text"`
			} `json:"path"`
			Lines struct {
				Text string `json:"text"`
			} `json:"lines"`
			LineNumber int `json:"line_number"`
			Submatches []struct {
				Start int `json:"start"`
				End   int `json:"end"`
			} `json:"submatches"`
		} `json:"data"`
	}

	var locations []wig.Location

	for row := range strings.SplitSeq(string(stdout), "\n") {
		row = strings.TrimSpace(row)
		if len(row) == 0 {
			continue
		}

		var match rgMatch
		if err := json.Unmarshal([]byte(row), &match); err != nil {
			continue
		}
		if match.Type != "match" {
			continue
		}

		char := 0
		if len(match.Data.Submatches) > 0 {
			char = match.Data.Submatches[0].Start
		}

		fullPath := filepath.Join(rootDir, match.Data.Path.Text)
		locations = append(locations, wig.Location{
			Text:     match.Data.Lines.Text,
			FilePath: fullPath,
			Line:     match.Data.LineNumber,
			Char:     char,
		})
	}

	return locations
}

// CmdRgUnderCursor extracts the word under the cursor (or selection),
// runs a ripgrep search, saves results to JSON, and opens the [rg]
// grouped buffer full screen.
func CmdRgUnderCursor(ctx wig.Context) {
	word := ""
	cur := wig.ContextCursorGet(ctx)
	if cur == nil || ctx.Buf == nil {
		return
	}

	if ctx.Buf.Selection != nil {
		word = wig.SelectionToString(ctx.Buf, ctx.Buf.Selection)
		wig.CmdNormalMode(ctx)
	} else {
		line := wig.CursorLine(ctx.Buf, cur)
		if line == nil || line.Value.IsEmpty() {
			ctx.Editor.EchoMessage("no word under cursor")
			return
		}
		if wig.CursorChClass(ctx.Buf, cur) == 0 {
			wig.CmdBackwardWord(ctx)
		}
		start, end := wig.TextObjectWord(ctx, true)
		if end+1 > start {
			line := wig.CursorLine(ctx.Buf, cur)
			if line != nil {
				word = string(line.Value.Range(start, end+1))
			}
		}
	}

	word = strings.TrimSpace(word)
	if word == "" {
		ctx.Editor.EchoMessage("no word under cursor")
		return
	}

	locations := rgSearchLocations(ctx, word)
	if len(locations) == 0 {
		ctx.Editor.EchoMessage("no results found for: " + word)
		return
	}

	if err := rgcollect.SaveResults(locations); err != nil {
		ctx.Editor.EchoMessage("failed to save results: " + err.Error())
	}

	rgcollect.InitGrouped(ctx, word, locations)
}

// CmdOpenSavedSearch reads the saved search results from JSON and opens
// the [rg] grouped buffer full screen.
func CmdOpenSavedSearch(ctx wig.Context) {
	locations := rgcollect.LoadResults()
	if len(locations) == 0 {
		ctx.Editor.EchoMessage("no saved search results")
		return
	}

	rgcollect.InitGrouped(ctx, "saved", locations)
}
