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

// CmdRg performs a ripgrep search for the query provided in ctx.Char (e.g. `:rg <pattern>`)
// or falls back to the word/selection under the cursor if no pattern is specified.
// Saves results to rg_search.json (for F11 grouped view), syncs to quickfix.json (for :copen / :cn / :cp),
// and opens the [rg] grouped buffer.
func CmdRg(ctx wig.Context) {
	query := strings.TrimSpace(ctx.Char)
	if query == "" {
		word, ok := wig.WordOrSelectionUnderCursor(ctx)
		if !ok {
			ctx.Editor.EchoMessage("no search pattern provided")
			return
		}
		query = word
	}

	locations := rgSearchLocations(ctx, query)
	if len(locations) == 0 {
		ctx.Editor.EchoMessage("no results found for: " + query)
		return
	}

	// 1. Save to rg_search.json for F11 grouped view
	if err := rgcollect.SaveResults(locations); err != nil {
		ctx.Editor.LogMessage("failed to save rg results: " + err.Error())
	}

	// 2. Parse & sync to quickfix.json for :copen, :cn, :cp
	SetQuickfixLocations(locations)

	// 3. Open grouped [rg] buffer view
	rgcollect.InitGrouped(ctx, query, locations)
}

// CmdRgUnderCursor extracts the word under the cursor (or selection),
// runs a ripgrep search, saves to rg_search.json & quickfix.json, and opens the [rg] grouped buffer.
func CmdRgUnderCursor(ctx wig.Context) {
	word, ok := wig.WordOrSelectionUnderCursor(ctx)
	if !ok {
		ctx.Editor.EchoMessage("no word under cursor")
		return
	}

	ctx.Char = word
	CmdRg(ctx)
}

// CmdOpenSavedSearch (F11) reads the saved search results from rg_search.json
// and opens the [rg] grouped buffer full screen, syncing quickfix state.
func CmdOpenSavedSearch(ctx wig.Context) {
	locations := rgcollect.LoadResults()
	if len(locations) == 0 {
		ctx.Editor.EchoMessage("no saved search results")
		return
	}

	SetQuickfixLocations(locations)
	rgcollect.InitGrouped(ctx, "saved", locations)
}
