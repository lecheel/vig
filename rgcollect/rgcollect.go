package rgcollect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
)

func Init(ctx wig.Context, title string, items []wig.Location) {
	if len(items) == 0 {
		ctx.Editor.EchoMessage("no items found")
		return
	}
	if len(ctx.Editor.Windows()) == 1 {
		wig.CmdWindowVSplit(ctx)
	}
	wig.CmdWindowNext(ctx)

	buf := wig.NewBuffer()
	buf.ResetLines()
	buf.FilePath = "[rgcollect " + title + "]"
	buf.Highlighter = &TestHighlighter{}

	buf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Enter": func(ctx wig.Context) {
				wig.VisitAtLine(ctx, buf, wig.VisitOptions{
					ParseLocation: true,
				})
			},
		},
	})

	ctx.Editor.Buffers = append(ctx.Editor.Buffers, buf)
	ctx.Buf = buf
	wig.EditorInst.ActiveWindow().VisitBuffer(ctx)

	for _, item := range items {
		v := fmt.Sprintf("%s:%d:%d %s", item.FilePath, item.Line, item.Char, strings.TrimSpace(item.Text))
		buf.Append(v)
	}

	wig.CmdWindowNext(ctx)
	wig.VisitAtLine(ctx, buf, wig.VisitOptions{
		Movement:      wig.CmdGotoLine0,
		ParseLocation: true,
	})
	wig.SetVisitSource(buf)
}

type TestHighlighter struct{}

func (h *TestHighlighter) Build()                          {}
func (h *TestHighlighter) TextChanged(wig.EventTextChange) {}
func (h *TestHighlighter) HighlightLine(lineNum int) []wig.Span {
	return nil
}

// ── Grouped rg search results (full screen, no split) ──

// RgResult is a single search result serialized to JSON.
//
// MatchStart / MatchEnd are rune offsets within Text (0-based, half-open).
// They delimit the ripgrep-reported match span and are used by the
// replace flow to splice the replacement at exactly the right place,
// bottom-up / right-to-left so no edit shifts the coordinates of another.
//
// Excluded is session state: it is not persisted, since F11 recall is
// meant to run a fresh search. Space in browse mode flips it; the
// replace apply skips excluded results.
type RgResult struct {
	FilePath   string `json:"file_path"`
	Line       int    `json:"line"`
	Char       int    `json:"char"`
	MatchStart int    `json:"match_start,omitempty"`
	MatchEnd   int    `json:"match_end,omitempty"`
	Text       string `json:"text"`
	Excluded   bool   `json:"-"`
}

// rgLineEntry maps a buffer line number to its semantic kind.
//
//	kind: 0 = blank/title, 1 = file header, 2 = result line
type rgLineEntry struct {
	kind      int
	resultIdx int
	filePath  string
}

var rgState = struct {
	results []RgResult
	lineMap map[int]rgLineEntry
}{
	lineMap: make(map[int]rgLineEntry),
}
var rgMutex sync.Mutex

// rgLoadedResults holds the full RgResult set (with match spans) most
// recently read by LoadResults. LoadResults' public signature only
// returns []wig.Location for compatibility with existing callers, so
// InitGrouped consults this package var — indexed the same as the
// returned locations — to recover MatchStart/MatchEnd on the "saved"
// recall path instead of falling back to a zero-width match.
var rgLoadedResults []RgResult

// RgHighlighter provides syntax highlighting for the [rg] grouped buffer.
type RgHighlighter struct {
	Buf     *wig.Buffer
	LineMap map[int]rgLineEntry
}

func (h *RgHighlighter) Build()                          {}
func (h *RgHighlighter) TextChanged(wig.EventTextChange) {}

func (h *RgHighlighter) HighlightLine(lineNum int) []wig.Span {
	if h.Buf == nil {
		return nil
	}
	line := wig.CursorLineByNum(h.Buf, lineNum)
	if line == nil {
		return nil
	}
	text := line.Value.String()
	runes := []rune(text)
	lineLen := uint16(len(runes))
	if lineLen == 0 {
		return nil
	}

	entry, ok := h.LineMap[lineNum]
	if !ok {
		return nil
	}

	switch entry.kind {
	case 0: // title
		return []wig.Span{{
			StartCol: 0,
			EndCol:   lineLen,
			Style:    wig.Color("ui.text.focus"), // White
		}}
	case 1: // file header
		// Dim the header when every result in that file is excluded.
		if entry.filePath != "" && rgAllExcludedInFile(entry.filePath) {
			return []wig.Span{{
				StartCol: 0,
				EndCol:   lineLen,
				Style:    rgDimStyle(),
			}}
		}
		return []wig.Span{{
			StartCol: 0,
			EndCol:   lineLen,
			Style:    wig.Color("ui.text.directory"), // Blue
		}}
	case 2: // result line
		// Locate the "N:\t" prefix so we can compute the display column of
		// the match span (rune offsets within the original text map to
		// rune offsets within the display line, offset by the prefix).
		tabIdx := -1
		for i, r := range runes {
			if r == '\t' {
				tabIdx = i
				break
			}
		}
		if tabIdx < 0 {
			return nil
		}
		prefixRunes := uint16(tabIdx + 1) // N digits + ':' + '\t'

		var result RgResult
		if entry.resultIdx >= 0 && entry.resultIdx < len(rgState.results) {
			result = rgState.results[entry.resultIdx]
		}

		if result.Excluded {
			return []wig.Span{{
				StartCol: 0,
				EndCol:   lineLen,
				Style:    rgDimStyle(),
			}}
		}

		spans := []wig.Span{{
			StartCol: 0,
			EndCol:   prefixRunes,
			Style:    wig.Color("comment"), // Green prefix
		}}

		matchStart := prefixRunes + uint16(result.MatchStart)
		matchEnd := prefixRunes + uint16(result.MatchEnd)
		if matchEnd > lineLen {
			matchEnd = lineLen
		}
		if matchStart < prefixRunes {
			matchStart = prefixRunes
		}
		if matchStart > lineLen {
			matchStart = lineLen
		}

		// In replace mode, once a non-empty replacement has been typed,
		// the buffer content has already been spliced by rgRefreshPreview
		// to look like:  <prefix><before><match><replacement><after>.
		// Draw the original match struck through and the replacement
		// text as a bold green block immediately after it — live, with
		// no separate preview toggle.
		inPreview := rgRSP.phase == rgPhaseReplace &&
			len(rgRSP.replacement) > 0

		if inPreview {
			if matchEnd > matchStart {
				spans = append(spans, wig.Span{
					StartCol: matchStart,
					EndCol:   matchEnd,
					Style:    rgStruckStyle(),
				})
			}
			replLen := uint16(len(rgRSP.replacement))
			replStart := matchEnd
			replEnd := replStart + replLen
			if replEnd > lineLen {
				replEnd = lineLen
			}
			if replEnd > replStart {
				spans = append(spans, wig.Span{
					StartCol: replStart,
					EndCol:   replEnd,
					Style:    rgReplacementStyle(),
				})
			}
		} else {
			if matchEnd > matchStart {
				spans = append(spans, wig.Span{
					StartCol: matchStart,
					EndCol:   matchEnd,
					Style:    rgMatchStyle(),
				})
			}
		}
		return spans
	}
	return nil
}

// InitGrouped opens a full-screen [rg] buffer with grouped search results.
// No split is created — the buffer replaces the current view.
func InitGrouped(ctx wig.Context, title string, locations []wig.Location) {
	rgMutex.Lock()
	defer rgMutex.Unlock()

	// Build results. MatchEnd is derived from the query string when it is
	// available (title == query for a live search); for the "saved" recall
	// path we leave MatchEnd == MatchStart and the replace flow will fall
	// back to a line-level search.
	queryRunes := 0
	if title != "" && title != "saved" {
		queryRunes = len([]rune(title))
	}
	// On the "saved" recall path, rgLoadedResults (set by LoadResults)
	// carries the real MatchStart/MatchEnd that were persisted to disk;
	// for a live search there's nothing loaded, so fall back to the
	// query-length-derived span as before.
	useLoaded := title == "saved" && len(rgLoadedResults) == len(locations)

	results := make([]RgResult, 0, len(locations))
	for i, loc := range locations {
		matchStart := loc.Char
		matchEnd := matchStart + queryRunes
		if useLoaded {
			matchStart = rgLoadedResults[i].MatchStart
			matchEnd = rgLoadedResults[i].MatchEnd
		}
		// Strip any trailing newline/CR once, here, so every later
		// consumer (initial render, rgRefreshPreview's browse-content
		// branch, the preview splice) works from clean text. Leaving
		// this untrimmed let a stray "\n" get joined into the buffer's
		// line-join on every replace-mode refresh, spawning an extra
		// blank line each time — compounding on repeated Tab/Esc.
		text := strings.TrimSuffix(strings.TrimSuffix(loc.Text, "\n"), "\r")
		results = append(results, RgResult{
			FilePath:   loc.FilePath,
			Line:       loc.Line,
			Char:       loc.Char,
			MatchStart: matchStart,
			MatchEnd:   matchEnd,
			Text:       text,
		})
	}
	rgState.results = results
	if title != "saved" {
		_ = saveResultsToFile(results)
	}

	// Find or create [rg] buffer
	buf := ctx.Editor.BufferFindByFilePath("[rg]", false)
	if buf == nil {
		buf = wig.NewBuffer()
		buf.FilePath = "[rg]"
		ctx.Editor.Buffers = append(ctx.Editor.Buffers, buf)
	}

	buf.ResetLines()

	// Build buffer content and lineMap
	lineMap := make(map[int]rgLineEntry)
	lineNum := 0

	rootDir := ctx.Editor.Projects.GetRoot()

	// Title line
	buf.Append(fmt.Sprintf("ripgrep search results for '%s' in %s", title, rootDir))
	lineMap[lineNum] = rgLineEntry{kind: 0}
	lineNum++

	// Blank line after title
	buf.Append("")
	lineMap[lineNum] = rgLineEntry{kind: 0}
	lineNum++

	resultIdx := 0
	var currentFile string
	for _, r := range results {
		if r.FilePath != currentFile {
			// Blank line between file groups (except before first group)
			if resultIdx > 0 {
				buf.Append("")
				lineMap[lineNum] = rgLineEntry{kind: 0}
				lineNum++
			}
			// File header
			buf.Append(r.FilePath)
			lineMap[lineNum] = rgLineEntry{kind: 1, filePath: r.FilePath}
			lineNum++
			currentFile = r.FilePath
		}

		// r.Text is already newline/CR-trimmed at construction above, so
		// MatchStart/MatchEnd offsets map exactly onto the displayed text;
		// the highlighter relies on this to draw the match span correctly.
		buf.Append(fmt.Sprintf("%d:\t%s", r.Line, r.Text))
		lineMap[lineNum] = rgLineEntry{kind: 2, resultIdx: resultIdx}
		lineNum++
		resultIdx++
	}

	rgState.lineMap = lineMap

	// Set highlighter
	buf.Highlighter = &RgHighlighter{Buf: buf, LineMap: lineMap}

	// Browse-mode key handler: Enter opens, Tab enters replace, Space
	// toggles inclusion, u undoes the last apply.
	rgInstallBrowseHandler(buf)

	// Visit buffer (current window, full screen)
	ctx.Buf = buf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: 2, Char: 0})
	wig.SetVisitSource(buf)
}

// CmdRgEnter is the Enter handler for the [rg] grouped buffer.
// Opens the file at the cursor's result location in the current window.
func CmdRgEnter(ctx wig.Context) {
	rgMutex.Lock()
	defer rgMutex.Unlock()

	cur := wig.ContextCursorGet(ctx)
	entry, ok := rgState.lineMap[cur.Line]
	if !ok {
		return
	}

	switch entry.kind {
	case 2: // result line
		if entry.resultIdx >= len(rgState.results) {
			return
		}
		result := rgState.results[entry.resultIdx]
		targetBuf, err := ctx.Editor.OpenFile(result.FilePath)
		if err != nil {
			ctx.Editor.EchoMessage("Cannot open: " + err.Error())
			return
		}
		ctx.Buf = targetBuf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
			Line: result.Line - 1,
			Char: result.Char,
		})
		wig.CmdCursorCenter(ctx)
	case 1: // filename header
		targetBuf, err := ctx.Editor.OpenFile(entry.filePath)
		if err != nil {
			ctx.Editor.EchoMessage("Cannot open: " + err.Error())
			return
		}
		ctx.Buf = targetBuf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
			Line: 0,
			Char: 0,
		})
	case 0: // blank/title — do nothing
	}
}

// visitRgCollectLine is the registered visit handler for [rgcollect ...] buffers.
// It automatically skips blank lines when navigating with :cn or :cp.
func visitRgCollectLine(ctx wig.Context, sourceBuf *wig.Buffer, movement func(wig.Context)) bool {
	if !strings.HasPrefix(sourceBuf.FilePath, "[rgcollect ") {
		return false
	}

	var sourceWin *wig.Window
	for _, win := range ctx.Editor.Windows() {
		if win.Buffer() == sourceBuf {
			sourceWin = win
			break
		}
	}

	if sourceWin == nil {
		sourceWin = ctx.Editor.ActiveWindow()
	}

	bufCur := wig.WindowCursorGet(sourceWin, sourceBuf)
	startLine := bufCur.Line
	maxLines := sourceBuf.Lines.Len

	if movement != nil {
		nctx := ctx.Editor.NewContext()
		nctx.Buf = sourceBuf
		nctx.Win = sourceWin

		movement(nctx)
		newLine := bufCur.Line

		found := false
		if newLine > startLine {
			for l := newLine; l < maxLines; l++ {
				line := wig.CursorLineByNum(sourceBuf, l)
				if line != nil && !line.Value.IsEmpty() {
					bufCur.Line = l
					bufCur.Char = 0
					found = true
					break
				}
			}
			if !found {
				bufCur.Line = startLine
				ctx.Editor.EchoMessage("No more search results")
				return true
			}
		} else if newLine < startLine {
			for l := newLine; l >= 0; l-- {
				line := wig.CursorLineByNum(sourceBuf, l)
				if line != nil && !line.Value.IsEmpty() {
					bufCur.Line = l
					bufCur.Char = 0
					found = true
					break
				}
			}
			if !found {
				bufCur.Line = startLine
				ctx.Editor.EchoMessage("No earlier search results")
				return true
			}
		} else {
			line := wig.CursorLineByNum(sourceBuf, newLine)
			if line == nil || line.Value.IsEmpty() {
				return true
			}
		}
	}

	line := wig.CursorLineByNum(sourceBuf, bufCur.Line)
	if line == nil {
		return true
	}

	filename, lineNum, chNum := wig.ParseFileLocation(line.Value.String(), 0)
	if filename == "" {
		ctx.Editor.EchoMessage("no file path found under cursor")
		return true
	}

	targetBuf, err := ctx.Editor.OpenFile(filename)
	if err != nil {
		ctx.Editor.EchoMessage("Cannot open: " + err.Error())
		return true
	}

	ctx.Buf = targetBuf
	ctx.Win = sourceWin
	sourceWin.VisitBuffer(ctx, wig.Cursor{
		Line: lineNum - 1,
		Char: chNum,
	})
	wig.CmdCursorCenter(ctx)

	return true
}

// visitLineGrouped is the registered visit handler for [rg] buffers.
// It looks up the lineMap to find result lines (kind == 2), automatically
// skipping blank/title lines (kind == 0) and filename headers (kind == 1)
// when navigating with :cn or :cp.
func visitLineGrouped(ctx wig.Context, sourceBuf *wig.Buffer, movement func(wig.Context)) bool {
	if sourceBuf.FilePath != "[rg]" {
		return false
	}

	rgMutex.Lock()
	defer rgMutex.Unlock()

	// Find the window containing the source buffer
	var sourceWin *wig.Window
	for _, win := range ctx.Editor.Windows() {
		if win.Buffer() == sourceBuf {
			sourceWin = win
			break
		}
	}

	// If [rg] is not in a window, use the active window to get/update its cursor.
	// The active window just opened a file from [rg], so it still has the cursor.
	if sourceWin == nil {
		sourceWin = ctx.Editor.ActiveWindow()
	}

	bufCur := wig.WindowCursorGet(sourceWin, sourceBuf)
	startLine := bufCur.Line
	maxLines := sourceBuf.Lines.Len

	if movement != nil {
		nctx := ctx.Editor.NewContext()
		nctx.Buf = sourceBuf
		nctx.Win = sourceWin

		movement(nctx)
		newLine := bufCur.Line

		found := false
		if newLine > startLine {
			// Moving forward (e.g. :cn / CmdCursorLineDown) — find next result line
			for l := newLine; l < maxLines; l++ {
				if entry, ok := rgState.lineMap[l]; ok && entry.kind == 2 {
					bufCur.Line = l
					bufCur.Char = 0
					found = true
					break
				}
			}
			if !found {
				bufCur.Line = startLine
				ctx.Editor.EchoMessage("No more search results")
				return true
			}
		} else if newLine < startLine {
			// Moving backward (e.g. :cp / CmdCursorLineUp) — find previous result line
			for l := newLine; l >= 0; l-- {
				if entry, ok := rgState.lineMap[l]; ok && entry.kind == 2 {
					bufCur.Line = l
					bufCur.Char = 0
					found = true
					break
				}
			}
			if !found {
				bufCur.Line = startLine
				ctx.Editor.EchoMessage("No earlier search results")
				return true
			}
		} else {
			// Cursor didn't move — search forward from current line
			for l := newLine; l < maxLines; l++ {
				if entry, ok := rgState.lineMap[l]; ok && entry.kind == 2 {
					bufCur.Line = l
					bufCur.Char = 0
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
	}

	entry, ok := rgState.lineMap[bufCur.Line]
	if !ok || entry.kind != 2 {
		return true
	}

	if entry.resultIdx >= len(rgState.results) {
		return true
	}

	result := rgState.results[entry.resultIdx]
	targetBuf, err := ctx.Editor.OpenFile(result.FilePath)
	if err != nil {
		ctx.Editor.EchoMessage("Cannot open: " + err.Error())
		return true
	}
	ctx.Buf = targetBuf
	ctx.Win = sourceWin
	sourceWin.VisitBuffer(ctx, wig.Cursor{
		Line: max(result.Line-1, 0),
		Char: result.Char,
	})
	ctx.Editor.EchoMessage(fmt.Sprintf("[%d/%d matches] %s", entry.resultIdx+1, len(rgState.results), strings.TrimSpace(result.Text)))
	wig.CmdCursorCenter(ctx)

	return true
}

func saveResultsToFile(results []RgResult) error {
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "wig")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "rg_search.json")
	return os.WriteFile(path, data, 0644)
}

// SaveResults serializes search results to ~/.config/wig/rg_search.json.
//
// When rgState.results is already populated and matches locations (e.g. after
// InitGrouped), its rich match spans (MatchStart/MatchEnd) are preserved.
// If SaveResults is called before InitGrouped or with new locations, it
// falls back to constructing RgResults from locations so persistence is
// never lost or overwritten with an empty slice.
func SaveResults(locations []wig.Location) error {
	rgMutex.Lock()
	defer rgMutex.Unlock()

	var results []RgResult
	matchesState := len(rgState.results) > 0 &&
		(len(locations) == 0 || (len(rgState.results) == len(locations) && rgState.results[0].FilePath == locations[0].FilePath && rgState.results[0].Line == locations[0].Line))

	if matchesState {
		results = make([]RgResult, len(rgState.results))
		copy(results, rgState.results)
	} else if len(locations) > 0 {
		results = make([]RgResult, len(locations))
		for i, loc := range locations {
			text := strings.TrimSuffix(strings.TrimSuffix(loc.Text, "\n"), "\r")
			results[i] = RgResult{
				FilePath:   loc.FilePath,
				Line:       loc.Line,
				Char:       loc.Char,
				MatchStart: loc.Char,
				MatchEnd:   loc.Char,
				Text:       text,
			}
		}
	} else {
		results = make([]RgResult, len(rgState.results))
		copy(results, rgState.results)
	}

	return saveResultsToFile(results)
}

// LoadResults reads saved search results from ~/.config/wig/rg_search.json.
//
// It also stashes the full decoded RgResult slice (with MatchStart/
// MatchEnd) in rgLoadedResults, indexed identically to the returned
// []wig.Location, so InitGrouped can recover the real match spans on the
// "saved" recall path instead of guessing a zero-width one.
func LoadResults() []wig.Location {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "wig", "rg_search.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var results []RgResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}
	rgLoadedResults = results
	locations := make([]wig.Location, len(results))
	for i, r := range results {
		locations[i] = wig.Location{
			FilePath: r.FilePath,
			Line:     r.Line,
			Char:     r.Char,
			Text:     r.Text,
		}
	}
	return locations
}

// rgDimStyle is applied to excluded result rows and to file headers whose
// whole group has been excluded. It applies strikethrough so items marked
// by Space (SPC) clearly display as struck out.
func rgDimStyle() tcell.Style {
	style := tcell.StyleDefault.Foreground(tcell.ColorGray)
	if s, ok := wig.FindColor("comment"); ok {
		style = s
	}
	return style.StrikeThrough(true)
}

// rgMatchStyle picks the span style for a match when preview is off:
//
//   - browse:                 yellow background (the "these match" cue)
//   - replace, preview off:   orange background (these are about to
//     change)
//
// Once preview is on, HighlightLine uses rgStruckStyle for the original
// match and rgReplacementStyle for the spliced-in replacement instead.
func rgMatchStyle() tcell.Style {
	if rgRSP.phase == rgPhaseReplace {
		return tcell.StyleDefault.
			Background(tcell.ColorOrange).
			Foreground(tcell.ColorBlack)
	}
	return tcell.StyleDefault.
		Background(tcell.ColorYellow).
		Foreground(tcell.ColorBlack)
}

// rgStruckStyle is the "this match is about to be rewritten" style:
// maroon background, white foreground, strikethrough. Applied to the
// original match span when preview is on so the user sees the bytes
// they are replacing, and can still read them.
func rgStruckStyle() tcell.Style {
	return tcell.StyleDefault.
		Background(tcell.ColorMaroon).
		Foreground(tcell.ColorWhite).
		StrikeThrough(true)
}

// rgReplacementStyle is the "this is the new text" style: bold green on
// a dark green background, drawn immediately after the struck-through
// match so the swap reads left-to-right.
func rgReplacementStyle() tcell.Style {
	return tcell.StyleDefault.
		Background(tcell.ColorDarkGreen).
		Foreground(tcell.ColorWhite).
		Bold(true)
}

// rgAllExcludedInFile reports whether every result belonging to the given
// file is currently excluded.
func rgAllExcludedInFile(filePath string) bool {
	seen := false
	for _, r := range rgState.results {
		if r.FilePath != filePath {
			continue
		}
		seen = true
		if !r.Excluded {
			return false
		}
	}
	return seen
}

func init() {
	wig.RegisterVisitHandler(visitLineGrouped)
	wig.RegisterVisitHandler(visitRgCollectLine)
}
