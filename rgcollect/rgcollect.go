package rgcollect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/firstrow/wig"
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
type RgResult struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Char     int    `json:"char"`
	Text     string `json:"text"`
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
		return []wig.Span{{
			StartCol: 0,
			EndCol:   lineLen,
			Style:    wig.Color("ui.text.directory"), // Blue
		}}
	case 2: // result line
		colonIdx := -1
		for i, r := range runes {
			if r == ':' {
				colonIdx = i
				break
			}
			if !unicode.IsDigit(r) {
				break
			}
		}
		if colonIdx > 0 {
			return []wig.Span{{
				StartCol: 0,
				EndCol:   uint16(colonIdx + 1),
				Style:    wig.Color("comment"), // Green
			}}
		}
	}
	return nil
}

// InitGrouped opens a full-screen [rg] buffer with grouped search results.
// No split is created — the buffer replaces the current view.
func InitGrouped(ctx wig.Context, title string, locations []wig.Location) {
	rgMutex.Lock()
	defer rgMutex.Unlock()

	// Build results
	results := make([]RgResult, 0, len(locations))
	for _, loc := range locations {
		results = append(results, RgResult{
			FilePath: loc.FilePath,
			Line:     loc.Line,
			Char:     loc.Char,
			Text:     loc.Text,
		})
	}
	rgState.results = results

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

		// Result line
		text := strings.TrimSpace(r.Text)
		buf.Append(fmt.Sprintf("%d:\t%s", r.Line, text))
		lineMap[lineNum] = rgLineEntry{kind: 2, resultIdx: resultIdx}
		lineNum++
		resultIdx++
	}

	rgState.lineMap = lineMap

	// Set highlighter
	buf.Highlighter = &RgHighlighter{Buf: buf, LineMap: lineMap}

	// Set key handler (DefaultKeyHandler + Enter override)
	buf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Enter": CmdRgEnter,
			"l": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				hl, ok := ctx.Buf.Highlighter.(*RgHighlighter)
				if !ok {
					return
				}
				for i := cur.Line + 1; i < ctx.Buf.Lines.Len; i++ {
					if entry, ok := hl.LineMap[i]; ok && entry.kind == 1 {
						cur.Line = i
						cur.Char = 0
						wig.CmdCursorCenter(ctx)
						return
					}
				}
			},
			"L": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				hl, ok := ctx.Buf.Highlighter.(*RgHighlighter)
				if !ok {
					return
				}
				for i := cur.Line - 1; i >= 0; i-- {
					if entry, ok := hl.LineMap[i]; ok && entry.kind == 1 {
						cur.Line = i
						cur.Char = 0
						wig.CmdCursorCenter(ctx)
						return
					}
				}
			},
		},
	})

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

// SaveResults serializes search results to ~/.config/wig/rg_search.json
func SaveResults(locations []wig.Location) error {
	results := make([]RgResult, len(locations))
	for i, loc := range locations {
		results[i] = RgResult{
			FilePath: loc.FilePath,
			Line:     loc.Line,
			Char:     loc.Char,
			Text:     loc.Text,
		}
	}
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "wig")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "rg_search.json")
	return os.WriteFile(path, data, 0644)
}

// LoadResults reads saved search results from ~/.config/wig/rg_search.json
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

func init() {
	wig.RegisterVisitHandler(visitLineGrouped)
	wig.RegisterVisitHandler(visitRgCollectLine)
}
