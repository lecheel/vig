package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

// QuickfixEntry is a single diagnostic serialized to JSON for persistence.
type QuickfixEntry struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"` // 0-indexed
	Char     int    `json:"char"` // 0-indexed
	Message  string `json:"message"`
}

// QuickfixHighlighter colors the filepath:line:col prefix in [quickfix] lines.
type QuickfixHighlighter struct {
	Buf *wig.Buffer
}

func (h *QuickfixHighlighter) Build()                          {}
func (h *QuickfixHighlighter) TextChanged(wig.EventTextChange) {}

func (h *QuickfixHighlighter) HighlightLine(lineNum int) []wig.Span {
	if h.Buf == nil {
		return nil
	}
	line := wig.CursorLineByNum(h.Buf, lineNum)
	if line == nil {
		return nil
	}
	text := line.Value.String()
	spaceIdx := strings.Index(text, " ")
	if spaceIdx > 0 {
		return []wig.Span{{
			StartCol: 0,
			EndCol:   uint16(spaceIdx),
			Style:    wig.Color("ui.linenr"),
		}}
	}
	return nil
}

// quickfixState holds the parsed entries for the [quickfix] buffer.
var quickfixState struct {
	entries []QuickfixEntry
	lineMap map[int]int // buffer line → entries index
}

// saveQuickfixResults serializes quickfix entries to
// quickfix.json for persistence across sessions.
func saveQuickfixResults(entries []QuickfixEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	dir := wig.GetConfigDir()
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "quickfix.json")
	return os.WriteFile(path, data, 0644)
}

// loadQuickfixResults reads saved quickfix entries from
// quickfix.json.
func loadQuickfixResults() []QuickfixEntry {
	path := filepath.Join(wig.GetConfigDir(), "quickfix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []QuickfixEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// populateQuickfixBuffer fills the [quickfix] buffer content from entries
// and returns the lineMap and highlighter.
func populateQuickfixBuffer(buf *wig.Buffer, entries []QuickfixEntry) map[int]int {
	buf.ResetLines()
	lineMap := make(map[int]int)

	buf.Append(fmt.Sprintf("Quickfix List — %d entries (q/Esc to close)", len(entries)))
	lineMap[0] = -1 // title line

	for i, e := range entries {
		line := fmt.Sprintf("%s:%d:%d %s",
			e.FilePath,
			e.Line+1, // 0-indexed → 1-indexed for display
			e.Char,
			e.Message,
		)
		buf.Append(line)
		lineMap[i+1] = i
	}

	return lineMap
}

// CmdQuickfixOpen opens the quickfix view (split window or popup) for the current
// quickfix entries, or loads previously saved entries from ~/.config/wig/quickfix.json.
// Navigate with :cn/:cp or Enter.
func CmdQuickfixOpen(ctx wig.Context) {
	entries := quickfixState.entries
	if len(entries) == 0 {
		entries = loadQuickfixResults()
		if len(entries) == 0 {
			ctx.Editor.EchoMessage("No quickfix entries (run :rg <query> or use Ctrl-r in search picker)")
			return
		}
		quickfixState.entries = entries
	}

	openQuickfixView(ctx, entries)
}

// CmdLspDiagnosticsToQuickfix explicitly collects LSP diagnostics for the current buffer,
// saves them to quickfix.json, and opens the quickfix view.
func CmdLspDiagnosticsToQuickfix(ctx wig.Context) {
	if ctx.Buf == nil {
		ctx.Editor.EchoMessage("No active buffer")
		return
	}

	diags := ctx.Editor.Lsp.AllDiagnostics(ctx.Buf)
	if len(diags) == 0 {
		ctx.Editor.EchoMessage("No LSP diagnostics for current buffer")
		return
	}

	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Range.Start.Line != diags[j].Range.Start.Line {
			return diags[i].Range.Start.Line < diags[j].Range.Start.Line
		}
		return diags[i].Range.Start.Character < diags[j].Range.Start.Character
	})

	entries := make([]QuickfixEntry, len(diags))
	for i, d := range diags {
		entries[i] = QuickfixEntry{
			FilePath: ctx.Buf.FilePath,
			Line:     int(d.Range.Start.Line),
			Char:     int(d.Range.Start.Character),
			Message:  d.Message,
		}
	}

	if err := saveQuickfixResults(entries); err != nil {
		ctx.Editor.LogMessage("failed to save quickfix: " + err.Error())
	}

	quickfixState.entries = entries
	openQuickfixView(ctx, entries)
	ctx.Editor.EchoMessage(fmt.Sprintf("%d diagnostics loaded into quickfix", len(entries)))
}

// SetQuickfixLocations converts search locations into Quickfix entries,
// saves them to ~/.config/wig/quickfix.json, and syncs in-memory state for :copen / :cn / :cp.
func SetQuickfixLocations(locations []wig.Location) []QuickfixEntry {
	if len(locations) == 0 {
		return nil
	}

	entries := make([]QuickfixEntry, len(locations))
	for i, loc := range locations {
		line := loc.Line
		if line > 0 {
			line = line - 1 // convert 1-indexed to 0-indexed
		}
		entries[i] = QuickfixEntry{
			FilePath: loc.FilePath,
			Line:     line,
			Char:     loc.Char,
			Message:  strings.TrimSpace(loc.Text),
		}
	}

	if err := saveQuickfixResults(entries); err != nil {
		if wig.EditorInst != nil {
			wig.EditorInst.LogMessage("failed to save quickfix: " + err.Error())
		}
	}

	quickfixState.entries = entries
	return entries
}

// OpenLocationsInQuickfix converts search locations into Quickfix entries,
// saves them, sets up the visit source for :cn/:cp, and opens via popup or split panel.
func OpenLocationsInQuickfix(ctx wig.Context, locations []wig.Location) {
	entries := SetQuickfixLocations(locations)
	if len(entries) == 0 {
		ctx.Editor.EchoMessage("No locations to display")
		return
	}

	openQuickfixView(ctx, entries)
	ctx.Editor.EchoMessage(fmt.Sprintf("%d matches loaded into quickfix (navigate with :cn / :cp)", len(entries)))
}

func openQuickfixView(ctx wig.Context, entries []QuickfixEntry) {
	if strings.ToLower(ctx.Editor.Config.QuickfixView) == "popup" {
		openQuickfixPopup(ctx, entries)
	} else {
		openQuickfixBuffer(ctx, entries)
	}
}

func openQuickfixPopup(ctx wig.Context, entries []QuickfixEntry) {
	qfBuf := ctx.Editor.BufferFindByFilePath("[quickfix]", true)
	lineMap := populateQuickfixBuffer(qfBuf, entries)
	quickfixState.lineMap = lineMap
	qfBuf.Highlighter = &QuickfixHighlighter{Buf: qfBuf}
	wig.SetVisitSource(qfBuf)

	// Seed cursor position to first entry
	if cur := wig.WindowCursorGet(ctx.Editor.ActiveWindow(), qfBuf); cur != nil {
		cur.Line = 1
		cur.Char = 0
	}

	items := make([]ui.QuickfixItem, len(entries))
	for i, e := range entries {
		items[i] = ui.QuickfixItem{
			FilePath: e.FilePath,
			Line:     e.Line,
			Char:     e.Char,
			Message:  e.Message,
		}
	}

	ui.QuickfixPopupInit(ctx, items, func(item ui.QuickfixItem) {
		// Sync quickfix buffer line so subsequent :cn/:cp continues from this item
		for line, idx := range quickfixState.lineMap {
			if idx >= 0 && idx < len(quickfixState.entries) {
				entry := quickfixState.entries[idx]
				if entry.FilePath == item.FilePath && entry.Line == item.Line && entry.Char == item.Char {
					if cur := wig.WindowCursorGet(ctx.Editor.ActiveWindow(), qfBuf); cur != nil {
						cur.Line = line
						cur.Char = 0
					}
					break
				}
			}
		}

		targetBuf, err := ctx.Editor.OpenFile(item.FilePath)
		if err != nil {
			ctx.Editor.EchoMessage("Cannot open: " + err.Error())
			return
		}
		ctx.Buf = targetBuf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
			Line: item.Line,
			Char: item.Char,
		})
		wig.CmdCursorCenter(ctx)
	})
}

func openQuickfixBuffer(ctx wig.Context, entries []QuickfixEntry) {
	qfBuf := ctx.Editor.BufferFindByFilePath("[quickfix]", true)

	lineMap := populateQuickfixBuffer(qfBuf, entries)
	quickfixState.lineMap = lineMap

	qfBuf.Highlighter = &QuickfixHighlighter{Buf: qfBuf}

	qfBuf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Enter": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				entryIdx, ok := quickfixState.lineMap[cur.Line]
				if !ok || entryIdx < 0 || entryIdx >= len(quickfixState.entries) {
					return
				}
				e := quickfixState.entries[entryIdx]
				targetBuf, err := ctx.Editor.OpenFile(e.FilePath)
				if err != nil {
					ctx.Editor.EchoMessage("Cannot open: " + err.Error())
					return
				}
				ctx.Buf = targetBuf
				ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
					Line: e.Line,
					Char: e.Char,
				})
				wig.CmdCursorCenter(ctx)
			},
			"q": func(ctx wig.Context) {
				if len(ctx.Editor.Windows()) > 1 {
					wig.CmdWindowClose(ctx)
				} else {
					wig.CmdBufferCycle(ctx)
				}
			},
			"Esc": func(ctx wig.Context) {
				if len(ctx.Editor.Windows()) > 1 {
					wig.CmdWindowClose(ctx)
				} else {
					wig.CmdBufferCycle(ctx)
				}
			},
		},
	})

	if len(ctx.Editor.Windows()) == 1 {
		wig.CmdWindowVSplit(ctx)
		wig.CmdWindowNext(ctx)
	}

	ctx.Buf = qfBuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: 1, Char: 0})
	wig.SetVisitSource(qfBuf)
}

// visitQuickfixLine handles :cn/:cp navigation in the [quickfix] buffer.
func visitQuickfixLine(ctx wig.Context, sourceBuf *wig.Buffer, movement func(wig.Context)) bool {
	if sourceBuf.FilePath != "[quickfix]" {
		return false
	}

	if len(quickfixState.entries) == 0 {
		entries := loadQuickfixResults()
		if len(entries) == 0 {
			ctx.Editor.EchoMessage("No quickfix entries")
			return true
		}
		quickfixState.entries = entries
		lineMap := populateQuickfixBuffer(sourceBuf, entries)
		quickfixState.lineMap = lineMap
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

	if movement != nil {
		nctx := ctx.Editor.NewContext()
		nctx.Buf = sourceBuf
		nctx.Win = sourceWin
		movement(nctx)

		newLine := bufCur.Line
		if newLine > startLine {
			for l := newLine; l < sourceBuf.Lines.Len; l++ {
				if entryIdx, ok := quickfixState.lineMap[l]; ok && entryIdx >= 0 {
					bufCur.Line = l
					bufCur.Char = 0
					break
				}
			}
		} else if newLine < startLine {
			for l := newLine; l >= 0; l-- {
				if entryIdx, ok := quickfixState.lineMap[l]; ok && entryIdx >= 0 {
					bufCur.Line = l
					bufCur.Char = 0
					break
				}
			}
		}
	}

	entryIdx, ok := quickfixState.lineMap[bufCur.Line]
	if !ok || entryIdx < 0 || entryIdx >= len(quickfixState.entries) {
		return true
	}

	e := quickfixState.entries[entryIdx]
	targetBuf, err := ctx.Editor.OpenFile(e.FilePath)
	if err != nil {
		ctx.Editor.EchoMessage("Cannot open: " + err.Error())
		return true
	}

	ctx.Buf = targetBuf
	ctx.Win = sourceWin
	sourceWin.VisitBuffer(ctx, wig.Cursor{
		Line: e.Line,
		Char: e.Char,
	})
	ctx.Editor.EchoMessage(fmt.Sprintf("[%d/%d matches] %s", entryIdx+1, len(quickfixState.entries), e.Message))
	wig.CmdCursorCenter(ctx)

	return true
}

func init() {
	wig.RegisterVisitHandler(visitQuickfixLine)
}
