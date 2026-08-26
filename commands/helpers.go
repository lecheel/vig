package commands

import (
	"encoding/json"
	"strings"

	"github.com/firstrow/wig"
)

// findTagRoot returns the repository root for the given context.
func findTagRoot(ctx wig.Context) string {
	rootDir, err := ctx.Editor.Projects.FindRoot(ctx.Buf)
	if err != nil || rootDir == "" {
		rootDir = ctx.Editor.Projects.GetRoot()
	}
	return rootDir
}

// parseRgJSON parses the raw JSON output from `rg --json` into a slice of wig.Location.
// It skips non-match lines and extracts file path, line number, character offset, and text.
func parseRgJSON(output []byte) []wig.Location {
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
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var match rgMatch
		if err := json.Unmarshal([]byte(line), &match); err != nil {
			continue
		}
		if match.Type != "match" {
			continue
		}
		char := 0
		if len(match.Data.Submatches) > 0 {
			char = match.Data.Submatches[0].Start
		}
		locations = append(locations, wig.Location{
			Text:     match.Data.Lines.Text,
			FilePath: match.Data.Path.Text,
			Line:     match.Data.LineNumber,
			Char:     char,
		})
	}
	return locations
}
