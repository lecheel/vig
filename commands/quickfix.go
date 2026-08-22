package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firstrow/wig"
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

func (h *QuickfixHighlighter) ForRange(startLine, endLine uint32) *wig.HighlighterCursor {
	if h.Buf == nil {
		return nil
	}

	nodes := wig.List[wig.HighlighterNode]{}

	line := wig.CursorLineByNum(h.Buf, int(startLine))
	for lineNum := startLine; line != nil && lineNum <= endLine; lineNum++ {
		text := line.Value.String()
		lineLen := uint32(len([]rune(text)))

		if lineLen > 0 {
			spaceIdx := strings.Index(text, " ")
			if spaceIdx > 0 {
				nodes.PushBack(wig.HighlighterNode{
					NodeName:  "ui.linenr",
					StartLine: lineNum,
					StartChar: 0,
					EndLine:   lineNum,
					EndChar:   uint32(spaceIdx),
				})
			}
		}

		line = line.Next()
	}

	if nodes.First() == nil {
		return nil
	}
	return &wig.HighlighterCursor{Cursor: nodes.First()}
}

// quickfixState holds the parsed entries for the [quickfix] buffer.
var quickfixState struct {
	entries []QuickfixEntry
	lineMap map[int]int // buffer line → entries index
}

// saveQuickfixResults serializes quickfix entries to
// ~/.config/wig/quickfix.json for persistence across sessions.
func saveQuickfixResults(entries []QuickfixEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "wig")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "quickfix.json")
	return os.WriteFile(path, data, 0644)
}

// loadQuickfixResults reads saved quickfix entries from
// ~/.config/wig/quickfix.json.
func loadQuickfixResults() []QuickfixEntry {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "wig", "quickfix.json")
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

// CmdQuickfixOpen collects LSP diagnostics from the current buffer and
// displays them in a [quickfix] buffer. Results are persisted to
// ~/.config/wig/quickfix.json so :copen can reopen the last set.
// Navigate with :cn/:cp or Enter.
func CmdQuickfixOpen(ctx wig.Context) {
	diags := ctx.Editor.Lsp.AllDiagnostics(ctx.Buf)
	if len(diags) == 0 {
		// Try loading saved results
		entries := loadQuickfixResults()
		if len(entries) == 0 {
			ctx.Editor.EchoMessage("No diagnostics and no saved results")
			return
		}
		quickfixState.entries = entries
		openQuickfixBuffer(ctx, entries)
		ctx.Editor.EchoMessage("Loaded saved quickfix results")
		return
	}

	// Sort by line, then char
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Range.Start.Line != diags[j].Range.Start.Line {
			return diags[i].Range.Start.Line < diags[j].Range.Start.Line
		}
		return diags[i].Range.Start.Character < diags[j].Range.Start.Character
	})

	// Convert to entries
	entries := make([]QuickfixEntry, len(diags))
	for i, d := range diags {
		entries[i] = QuickfixEntry{
			FilePath: ctx.Buf.FilePath,
			Line:     int(d.Range.Start.Line),
			Char:     int(d.Range.Start.Character),
			Message:  d.Message,
		}
	}

	// Persist
	if err := saveQuickfixResults(entries); err != nil {
		ctx.Editor.LogMessage("failed to save quickfix: " + err.Error())
	}

	quickfixState.entries = entries
	openQuickfixBuffer(ctx, entries)
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
				if len(ctx.Editor.Windows) > 1 {
					wig.CmdWindowClose(ctx)
				} else {
					wig.CmdBufferCycle(ctx)
				}
			},
			"Esc": func(ctx wig.Context) {
				if len(ctx.Editor.Windows) > 1 {
					wig.CmdWindowClose(ctx)
				} else {
					wig.CmdBufferCycle(ctx)
				}
			},
		},
	})

	if len(ctx.Editor.Windows) == 1 {
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

	var sourceWin *wig.Window
	for _, win := range ctx.Editor.Windows {
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
	wig.CmdCursorCenter(ctx)

	return true
}

func init() {
	wig.RegisterVisitHandler(visitQuickfixLine)
}
