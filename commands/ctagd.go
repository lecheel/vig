package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

// CtagdSocketPath is the Unix domain socket used by the ctagd daemon.
const CtagdSocketPath = "/tmp/.ctagd.sock"

// ---------------------------------------------------------------------------
// JSON message types (see client.md / RustLSP Daemon: Editor Integration Spec)
// ---------------------------------------------------------------------------

type ctagdRequest struct {
	ID       string `json:"id"`
	Method   string `json:"method"`
	RepoRoot string `json:"repo_root"`
	File     string `json:"file,omitempty"`
	Content  string `json:"content,omitempty"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Symbol   string `json:"symbol,omitempty"`
	Query    string `json:"query,omitempty"`
}

type ctagdResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
}

type ctagdLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Display string `json:"display,omitempty"`
}

type ctagdSymbol struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	RelativePath string `json:"relative_path"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	Detail       string `json:"detail,omitempty"`
}

// ---------------------------------------------------------------------------
// Connection / transport
// ---------------------------------------------------------------------------

var ctagdIDCounter uint64

func ctagdNextID() string {
	id := atomic.AddUint64(&ctagdIDCounter, 1)
	return fmt.Sprintf("wig-%d-%d", time.Now().UnixNano(), id)
}

// ctagdIsConnected returns true if the daemon socket is reachable.
func ctagdIsConnected() bool {
	conn, err := net.DialTimeout("unix", CtagdSocketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ctagdSend sends an NDJSON request over the Unix socket. When wait is false
// the request is fire-and-forget (the response is not read), suitable for the
// `saved` method per Workflow 1 of the spec.
func ctagdSend(req ctagdRequest, wait bool) (*ctagdResponse, error) {
	conn, err := net.DialTimeout("unix", CtagdSocketPath, 2*time.Second)
	if err != nil {
		// Daemon auto-start: optionally spawn ctagd if the binary is available.
		if _, lookErr := exec.LookPath("ctagd"); lookErr == nil {
			startCmd := exec.Command("ctagd")
			_ = startCmd.Start()
			for i := 0; i < 20; i++ {
				time.Sleep(50 * time.Millisecond)
				conn, err = net.DialTimeout("unix", CtagdSocketPath, 500*time.Millisecond)
				if err == nil {
					break
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("ctagd: cannot connect to %s: %w", CtagdSocketPath, err)
		}
	}
	defer conn.Close()

	req.ID = ctagdNextID()
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, err
	}

	if !wait {
		return nil, nil
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var resp ctagdResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ctagdBufferText(buf *wig.Buffer) string {
	if buf == nil {
		return ""
	}
	var sb strings.Builder
	for line := buf.Lines.First(); line != nil; line = line.Next() {
		sb.WriteString(string(line.Value))
	}
	return sb.String()
}

func ctagdIsNullResult(resp *ctagdResponse) bool {
	if resp == nil || len(resp.Result) == 0 {
		return true
	}
	return string(resp.Result) == "null"
}

func ctagdJumpToLocation(ctx wig.Context, repoRoot string, loc ctagdLocation) {
	absPath := loc.File
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(repoRoot, loc.File)
	}
	ctx.Editor.LogMessage(fmt.Sprintf("ctagd: raw location %s line:%d col:%d", absPath, loc.Line, loc.Column))
	nbuf, err := ctx.Editor.OpenFile(absPath)
	if err != nil {
		ctx.Editor.EchoMessage("ctagd: cannot open " + absPath + ": " + err.Error())
		return
	}
	// Unlike the local ctags "tags" file (which is 1-based), ctagd is
	// LSP-backed and reports 0-based line/column numbers already matching
	// wig.Cursor's convention directly — no "-1" conversion here.
	line := loc.Line
	if line < 0 {
		line = 0
	}
	col := loc.Column
	if col < 0 {
		col = 0
	}
	ctx.Editor.LogMessage(fmt.Sprintf("ctagd: adjusted cursor line:%d col:%d", line, col))
	cursor := wig.Cursor{Line: line, Char: col}
	ctx.Buf = nbuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, cursor)
	wig.CmdCursorCenter(ctx.Editor.NewContext())
}

// ---------------------------------------------------------------------------
// API method commands (spec §3)
// ---------------------------------------------------------------------------

// CmdCtagdSaved notifies the daemon that a file has been written to disk.
// Fire-and-forget: runs in a goroutine and does not block the editor (spec §4 Workflow 1).
func CmdCtagdSaved(ctx wig.Context) {
	if ctx.Buf == nil || ctx.Buf.FilePath == "" || strings.HasPrefix(ctx.Buf.FilePath, "[") {
		return
	}
	repoRoot := findTagRoot(ctx)
	if repoRoot == "" {
		return
	}
	rel, err := filepath.Rel(repoRoot, ctx.Buf.FilePath)
	if err != nil {
		return
	}
	content := ctagdBufferText(ctx.Buf)
	go func() {
		_, _ = ctagdSend(ctagdRequest{
			Method:   "saved",
			RepoRoot: repoRoot,
			File:     rel,
			Content:  content,
		}, false)
	}()
}

// ctagdDefinitionAndJump queries the daemon for the definition of the symbol
// under the cursor, using file/line/column context to disambiguate between
// multiple symbols sharing the same name (spec §3.2). This is the
// context-aware counterpart to ctagdGotoAndJump, which only matches by name
// and can therefore land on the wrong occurrence of a common identifier.
// When echo is true, failures are reported via EchoMessage; otherwise it
// fails silently so callers can fall back to another source.
func ctagdDefinitionAndJump(ctx wig.Context, echo bool) bool {
	if ctx.Buf == nil || ctx.Buf.FilePath == "" || strings.HasPrefix(ctx.Buf.FilePath, "[") {
		if echo {
			ctx.Editor.EchoMessage("ctagd: no file")
		}
		return false
	}
	repoRoot := findTagRoot(ctx)
	if repoRoot == "" {
		if echo {
			ctx.Editor.EchoMessage("ctagd: no repo root")
		}
		return false
	}
	rel, err := filepath.Rel(repoRoot, ctx.Buf.FilePath)
	if err != nil {
		return false
	}
	cur := wig.ContextCursorGet(ctx)
	symbol, _ := wig.WordOrSelectionUnderCursor(ctx)
	resp, err := ctagdSend(ctagdRequest{
		Method:   "definition",
		RepoRoot: repoRoot,
		File:     rel,
		Line:     cur.Line,
		Column:   cur.Char,
		Symbol:   symbol,
	}, true)
	if err != nil {
		if echo {
			ctx.Editor.EchoMessage("ctagd: " + err.Error())
		}
		return false
	}
	if ctagdIsNullResult(resp) {
		if echo {
			ctx.Editor.EchoMessage("ctagd: no definition found")
		}
		return false
	}
	var loc ctagdLocation
	if err := json.Unmarshal(resp.Result, &loc); err != nil || loc.File == "" {
		if echo {
			ctx.Editor.EchoMessage("ctagd: no definition found")
		}
		return false
	}
	ctagdJumpToLocation(ctx, repoRoot, loc)
	return true
}

// CmdCtagdGotoDefinition queries the daemon for the definition of the symbol
// under the cursor (spec §3.2). The `symbol` field provides an instant
// SQLite fallback when the LSP server is cold.
func CmdCtagdGotoDefinition(ctx wig.Context) {
	ctagdDefinitionAndJump(ctx, true)
}

// ctagdGotoAndJump sends a `goto` request (spec §3.3) and jumps to the first
// matched location. Returns false when no location is found, allowing callers
// to fall back to the local tags file silently.
//
// NOTE: this is a name-only lookup with no file/line/column context, so if
// the daemon's index has multiple symbols named `word` (e.g. across files,
// or a common name like "New" / "Run" / "Init"), the daemon's own ranking
// decides which one comes back — wig has no say in it here. The debug logs
// below are to help diagnose "goes to wrong place" reports: check
// :messages for the exact query sent and the raw location received.
func ctagdGotoAndJump(ctx wig.Context, word string) bool {
	repoRoot := findTagRoot(ctx)
	if repoRoot == "" {
		ctx.Editor.LogMessage("ctagd goto: no repo root, word=" + word)
		return false
	}
	ctx.Editor.LogMessage(fmt.Sprintf("ctagd goto: query=%q repoRoot=%s", word, repoRoot))
	resp, err := ctagdSend(ctagdRequest{
		Method:   "goto",
		RepoRoot: repoRoot,
		Query:    word,
	}, true)
	if err != nil {
		ctx.Editor.LogMessage("ctagd goto: request error: " + err.Error())
		return false
	}
	if ctagdIsNullResult(resp) {
		ctx.Editor.LogMessage("ctagd goto: null result for query=" + word)
		return false
	}
	ctx.Editor.LogMessage("ctagd goto: raw result=" + string(resp.Result))
	var loc ctagdLocation
	if err := json.Unmarshal(resp.Result, &loc); err != nil || loc.File == "" {
		ctx.Editor.LogMessage(fmt.Sprintf("ctagd goto: unmarshal/empty file, err=%v", err))
		return false
	}
	ctx.Editor.LogMessage(fmt.Sprintf("ctagd goto: matched file=%s line=%d col=%d display=%q",
		loc.File, loc.Line, loc.Column, loc.Display))
	ctagdJumpToLocation(ctx, repoRoot, loc)
	return true
}

// CmdCtagdGoto jumps directly to a symbol by name (spec §3.3).
func CmdCtagdGoto(ctx wig.Context) {
	word := strings.TrimSpace(ctx.Char)
	if word == "" {
		w, ok := wig.WordOrSelectionUnderCursor(ctx)
		if !ok || w == "" {
			ctx.Editor.EchoMessage("ctagd: no symbol")
			return
		}
		word = w
	}
	if !ctagdGotoAndJump(ctx, word) {
		ctx.Editor.EchoMessage(fmt.Sprintf("ctagd: tag not found: %s", word))
	}
}

// CmdCtagdWorkspaceSymbols searches the repository for symbols matching a
// pattern and presents results in a picker (spec §3.4).
func CmdCtagdWorkspaceSymbols(ctx wig.Context) {
	query := strings.TrimSpace(ctx.Char)
	if query == "" {
		w, _ := wig.WordOrSelectionUnderCursor(ctx)
		query = w
	}
	if query == "" {
		ctx.Editor.EchoMessage("ctagd: no query")
		return
	}
	repoRoot := findTagRoot(ctx)
	if repoRoot == "" {
		ctx.Editor.EchoMessage("ctagd: no repo root")
		return
	}
	resp, err := ctagdSend(ctagdRequest{
		Method:   "workspace_symbols",
		RepoRoot: repoRoot,
		Query:    query,
	}, true)
	if err != nil {
		ctx.Editor.EchoMessage("ctagd: " + err.Error())
		return
	}
	if ctagdIsNullResult(resp) {
		ctx.Editor.EchoMessage("ctagd: no symbols found")
		return
	}
	var symbols []ctagdSymbol
	if err := json.Unmarshal(resp.Result, &symbols); err != nil {
		ctx.Editor.EchoMessage("ctagd: invalid response")
		return
	}
	if len(symbols) == 0 {
		ctx.Editor.EchoMessage("ctagd: no symbols found")
		return
	}
	items := make([]ui.PickerItem[ctagdSymbol], 0, len(symbols))
	for _, s := range symbols {
		display := fmt.Sprintf("[%s] %s  %s:%d", s.Kind, s.Name, s.RelativePath, s.Line+1)
		if s.Detail != "" {
			display += "  " + s.Detail
		}
		items = append(items, ui.PickerItem[ctagdSymbol]{Name: display, Value: s})
	}
	root := repoRoot
	action := func(p *ui.UiPicker[ctagdSymbol], i *ui.PickerItem[ctagdSymbol]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			return
		}
		s := i.Value
		absPath := s.RelativePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(root, s.RelativePath)
		}
		nbuf, err := ctx.Editor.OpenFile(absPath)
		if err != nil {
			ctx.Editor.EchoMessage("ctagd: cannot open: " + err.Error())
			return
		}
		line := s.Line
		if line < 0 {
			line = 0
		}
		col := s.Column
		if col < 0 {
			col = 0
		}
		cursor := wig.Cursor{Line: line, Char: col}
		ctx.Buf = nbuf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, cursor)
		wig.CmdCursorCenter(ctx.Editor.NewContext())
	}
	picker := ui.PickerInit(ctx.Editor, action, items)
	picker.SetTitle(fmt.Sprintf("ctagd: %s (%d)", query, len(symbols)))
}

// CmdCtagdStatus reports whether the ctagd daemon is reachable.
func CmdCtagdStatus(ctx wig.Context) {
	if ctagdIsConnected() {
		ctx.Editor.EchoMessage("ctagd: connected (" + CtagdSocketPath + ")")
		return
	}
	if _, err := exec.LookPath("ctagd"); err == nil {
		ctx.Editor.EchoMessage("ctagd: not running (binary found, socket missing at " + CtagdSocketPath + ")")
	} else {
		ctx.Editor.EchoMessage("ctagd: not installed (ctagd binary not in PATH)")
	}
}

// init registers the ctagd save hook so that every successful save
// automatically notifies the daemon (spec §4 Workflow 1).
func init() {
	wig.OnSaveHook = CmdCtagdSaved
}
