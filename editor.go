package wig

import (
	"github.com/gdamore/tcell/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type EditorConfig struct {
	Leader              string `toml:"leader"`
	CommentStyle        string `toml:"comment_style"` // "standard" (supports motions: gcc, gcj, gcip, gc$) or "simple" (direct gc toggle)
	Theme               string `toml:"theme"`
	ShowLineNumbers     bool   `toml:"show_line_numbers"`
	RelativeLineNumbers bool   `toml:"relative_line_numbers"`
	CurrentLineAbsolute bool   `toml:"current_line_absolute"` // If true, shows absolute line number on current line when relative is on
	FormatOnSave        bool   `toml:"format_on_save"`
	GitStatusView       string `toml:"git_status_view"` // "full" or "split"
	GitBlameView        string `toml:"git_blame_view"`  // "full" or "split"
	QuickfixView        string `toml:"quickfix_view"`   // "split" (default) or "popup"
	IndentGuides        bool   `toml:"indent_guides"`
	LspEnabled          bool   `toml:"lsp_enabled"`           // If false, LSP servers are never started and all LSP features are no-ops
	WhichKeyFormat      string `toml:"which_key_format"`      // "words", "camel", or "cmd"
	SameStatuslineColor bool   `toml:"same_statusline_color"` // If true, keeps statusline color consistent across modes (disables distinct insert color)
	StatuslineStyle     string `toml:"statusline_style"`
	FilePickerView      string `toml:"file_picker_view"` // "tree" (default) or "files"
	AutoSession         bool   `toml:"auto_session"`     // Automatically save/restore the last session on startup/quit
}

type View interface {
	SetContent(x, y int, str string, st tcell.Style)
	Size() (width, height int)
	Resize(x, y, width, height int)
}

type RenderPlane int

const (
	PlaneWin    RenderPlane = 0
	PlaneEditor RenderPlane = 1
)

type UiComponent interface {
	Mode() Mode
	Keymap() *KeyHandler
	Render(view View)
	Plane() RenderPlane
}

type Mark struct {
	Buf    *Buffer
	Cursor Cursor
}

type Context struct {
	Editor *Editor
	Buf    *Buffer
	Win    *Window
	Count  uint32
	Char   string
}

type AutocompleteFn func(Context) bool

// MarksPopupFactory allows the `ui` package to register a popup for marks
// without causing a circular import. Marks are global across all buffers.
var MarksPopupFactory func(ctx Context, marks map[rune]Mark)

// RegistersPopupFactory allows the `ui` package to register a popup for registers
// without causing a circular import.
var RegistersPopupFactory func(ctx Context)

var EditorInst *Editor

type Layout int

const (
	LayoutHorizontal Layout = 0
	LayoutVertical   Layout = 1
)

// SplitDir describes how a WinNode's children are arranged.
type SplitDir int

const (
	SplitNone       SplitDir = iota // leaf: a single Window
	SplitVertical                   // children side-by-side (left/right) — ":vs"
	SplitHorizontal                 // children stacked (top/bottom) — ":sp"
)

// WinNode is one node of the editor's recursive split tree. A leaf
// (Dir == SplitNone) wraps exactly one Window. A split node wraps two or
// more children arranged along Dir.
type WinNode struct {
	Dir      SplitDir
	Win      *Window
	Children []*WinNode
}

func LeafNode(w *Window) *WinNode { return &WinNode{Win: w} }
func leafNode(w *Window) *WinNode { return &WinNode{Win: w} }

// FindLeaf returns the leaf node wrapping win, or nil if win isn't in the tree.
func FindLeaf(node *WinNode, win *Window) *WinNode {
	if node == nil {
		return nil
	}
	if node.Dir == SplitNone {
		if node.Win == win {
			return node
		}
		return nil
	}
	for _, c := range node.Children {
		if f := FindLeaf(c, win); f != nil {
			return f
		}
	}
	return nil
}

func findLeaf(node *WinNode, win *Window) *WinNode { return FindLeaf(node, win) }

// RemoveLeaf removes win's leaf from the tree, collapsing any split node
// left with a single child. Returns the (possibly new) subtree root and
// whether win was found.
func RemoveLeaf(node *WinNode, win *Window) (*WinNode, bool) {
	if node == nil {
		return nil, false
	}
	if node.Dir == SplitNone {
		if node.Win == win {
			return nil, true
		}
		return node, false
	}
	for i, c := range node.Children {
		newC, found := RemoveLeaf(c, win)
		if !found {
			continue
		}
		if newC == nil {
			node.Children = append(node.Children[:i], node.Children[i+1:]...)
		} else {
			node.Children[i] = newC
		}
		switch len(node.Children) {
		case 0:
			return nil, true
		case 1:
			return node.Children[0], true // collapse 1-child split
		default:
			return node, true
		}
	}
	return node, false
}

func removeLeaf(node *WinNode, win *Window) (*WinNode, bool) { return RemoveLeaf(node, win) }

type Workspace struct {
	Num          int
	Windows      []*Window
	ActiveWindow *Window
	Root         *WinNode
	Layout       Layout
	// Files is the ordered history of every real file opened while this
	// workspace was active. A window only ever shows one buffer at a time,
	// so without this a workspace that had file1 -> file2 -> file3 opened
	// in the same window would only ever remember file3. See
	// Editor.recordWorkspaceFile.
	Files []string
	// ActiveSession is the name of the session currently loaded into
	// this workspace (if any). :mksession with no argument reuses this
	// name, falling back to the project root name.
	ActiveSession string
}

type Editor struct {
	View                View
	Keys                *KeyHandler
	Buffers             []*Buffer
	UiComponents        []UiComponent
	ExitCh              chan int
	RedrawCh            chan int
	ScreenSyncCh        chan int
	Layout              Layout
	Yanks               List[yank]
	Projects            ProjectManager
	Message             string // display in echo area
	Lsp                 *LspManager
	Events              *EventsManager
	AutocompleteTrigger AutocompleteFn
	Snippets            *SnippetsManager
	Config              EditorConfig
	Workspaces          []Workspace
	ActiveWorkspace     int
	LastRepeatableFn    func(Context)
	ActiveRegister      rune
	Marks               map[rune]Mark
}

func NewEditor(
	view View,
	keys *KeyHandler,
) *Editor {
	windows := []*Window{CreateWindow(nil)}
	workspaces := make([]Workspace, 10)
	workspaces[1] = Workspace{
		Num:          1,
		Windows:      windows,
		ActiveWindow: windows[0],
		Root:         leafNode(windows[0]),
		Layout:       LayoutVertical,
	}

	EditorInst = &Editor{
		View:            view,
		Keys:            keys,
		Buffers:         make([]*Buffer, 0, 32),
		Yanks:           List[yank]{},
		Layout:          LayoutVertical,
		Projects:        NewProjectManager(),
		ExitCh:          make(chan int),
		RedrawCh:        make(chan int, 10),
		ScreenSyncCh:    make(chan int),
		Events:          NewEventsManager(),
		Snippets:        NewSnippetsManager(),
		Workspaces:      workspaces,
		ActiveWorkspace: 1,
		Marks:           make(map[rune]Mark),
	}

	EditorInst.Lsp = NewLspManager(EditorInst)
	TreeSitterHighlighterGo(EditorInst)

	return EditorInst
}

func (e *Editor) Windows() []*Window {
	return e.Workspaces[e.ActiveWorkspace].Windows
}

func (e *Editor) SetWindows(w []*Window) {
	e.Workspaces[e.ActiveWorkspace].Windows = w
}

func (e *Editor) OpenFile(path string) (*Buffer, error) {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	if fbuf := e.BufferFindByFilePath(path, false); fbuf != nil {
		e.recordWorkspaceFile(path)
		e.showInActiveWindowIfEmpty(fbuf)
		return fbuf, nil
	}

	buf, err := BufferReadFile(path)
	if err != nil {
		// If the file doesn't exist, create a new empty buffer instead of failing.
		// This allows opening non-existent files from the command line (e.g. `wig newfile.txt`)
		// and from file pickers, behaving like standard editors.
		if !os.IsNotExist(err) {
			e.LogError(err)
			return nil, err
		}
		buf = NewBuffer()
		buf.FilePath = path
	}
	if len(e.Buffers) == 1 && e.Buffers[0].FilePath == "[No Name]" && !e.Buffers[0].Dirty {
		e.Buffers[0] = buf
		// Update any windows that were showing the old [No Name] buffer
		for _, win := range e.Windows() {
			if win.buf != nil && win.buf.FilePath == "[No Name]" {
				win.ShowBuffer(buf)
			}
		}
	} else {
		e.Buffers = append(e.Buffers, buf)
	}
	// The block above only reattaches windows editor-wide when the whole
	// editor happened to have exactly one buffer total (the startup case).
	// That leaves any window whose own buffer is nil or an unmodified
	// "[No Name]" placeholder — e.g. a freshly created workspace's window,
	// or a workspace seeded with its own placeholder buffer — never
	// getting shown the file it just opened. Fix that per-window instead
	// of per-editor.
	e.showInActiveWindowIfEmpty(buf)

	e.Lsp.DidOpen(buf)

	hl := TreeSitterHighlighterInitBuffer(e, buf)
	if hl != nil {
		buf.Highlighter = hl
	}

	// Broadcast event so GitGutterManager calculates signs for newly opened file
	e.Events.Broadcast(EventBufferReloaded{Buf: buf})
	e.recordWorkspaceFile(buf.FilePath)
	return buf, nil
}

// recordWorkspaceFile appends fp to the active workspace's opened-file
// history, used by WorkspaceCache.CaptureWorkspace to restore every file
// that was ever shown in the workspace, not just whatever buffer currently
// occupies a window. Existing entries move to the end so order reflects
// most-recent use. Special buffers ("[No Name]", "[Messages]", etc.) are
// skipped.
func (e *Editor) recordWorkspaceFile(fp string) {
	if fp == "" || strings.HasPrefix(fp, "[") {
		return
	}
	ws := e.GetActiveWorkspace()
	for i, f := range ws.Files {
		if f == fp {
			ws.Files = slices.Delete(ws.Files, i, i+1)
			break
		}
	}
	ws.Files = append(ws.Files, fp)
}

// showInActiveWindowIfEmpty shows buf in the active window of the active
// workspace when that window currently has no buffer at all, or only an
// unmodified "[No Name]" placeholder. This keeps opening a file from ever
// leaving a window with a nil buffer (see the nil-buf guard in
// HandleInput), without disturbing windows that already show real,
// unrelated content — e.g. WorkspaceCache.RestoreWorkspace intentionally
// loads background buffers via OpenFile without wanting them to steal the
// active window's buffer, which this leaves untouched since that window's
// buffer is neither nil nor an unmodified "[No Name]" at that point.
func (e *Editor) showInActiveWindowIfEmpty(buf *Buffer) {
	win := e.ActiveWindow()
	if win == nil {
		return
	}
	cur := win.Buffer()
	if cur == nil || (cur.FilePath == "[No Name]" && !cur.Dirty) {
		win.ShowBuffer(buf)
	}
}

func (e *Editor) NewContext() Context {
	return Context{
		Editor: e,
		Buf:    e.ActiveBuffer(),
		Win:    e.ActiveWindow(),
		Count:  0,
	}
}

// Find or create new buffer by its full file path
func (e *Editor) BufferFindByFilePath(fp string, create bool) *Buffer {
	for _, b := range e.Buffers {
		if b.FilePath == fp {
			return b
		}
	}

	if !create {
		return nil
	}

	b := NewBuffer()
	b.FilePath = fp
	e.Buffers = append(e.Buffers, b)

	return b
}

// Returns active window buffer.
// May return nil when the active workspace has no active window/buffer,
// e.g. transiently after ":q" closed the last window.
func (e *Editor) ActiveBuffer() *Buffer {
	win := e.ActiveWindow()
	if win == nil {
		return nil
	}
	return win.Buffer()
}

func (e *Editor) GetActiveWorkspace() *Workspace {
	return &e.Workspaces[e.ActiveWorkspace]
}

func (e *Editor) GetWorkspace(num int) *Workspace {
	return &e.Workspaces[num]
}

func (e *Editor) ActiveWindow() *Window {
	return e.Workspaces[e.ActiveWorkspace].ActiveWindow
}

func (e *Editor) SetActiveWindow(w *Window) {
	e.Workspaces[e.ActiveWorkspace].ActiveWindow = w
}

func (e *Editor) PushUi(c UiComponent) {
	e.UiComponents = append(e.UiComponents, c)
}

func (e *Editor) PopUi() {
	if len(e.UiComponents) > 0 {
		e.UiComponents = e.UiComponents[:len(e.UiComponents)-1]
	}
}

// PopUiComponent removes a specific UI component from the stack.
// This is required for WhichKey because commands executed from WhichKey
// can push their own UI components (Picker, CommandLine, etc.) before
// WhichKey.Close is called. Using PopUi in that case removes the newly
// pushed component instead of WhichKey, leaving WhichKey stuck on screen.
func (e *Editor) PopUiComponent(c UiComponent) {
	for i := len(e.UiComponents) - 1; i >= 0; i-- {
		if e.UiComponents[i] == c {
			e.UiComponents = append(e.UiComponents[:i], e.UiComponents[i+1:]...)
			break
		}
	}
}

func (e *Editor) Root() *WinNode {
	ws := e.GetActiveWorkspace()
	if ws == nil {
		return nil
	}
	return ws.Root
}

func (e *Editor) SetRoot(r *WinNode) {
	ws := e.GetActiveWorkspace()
	if ws != nil {
		ws.Root = r
	}
}

func (e *Editor) EnsureBufferIsVisible(b *Buffer) {
	ws := e.GetActiveWorkspace()
	for _, win := range ws.Windows {
		if win.Buffer() == b {
			return
		}
	}
	if len(ws.Windows) > 1 {
		ws.Windows[len(ws.Windows)-1].ShowBuffer(b)
		return
	}
	win := CreateWindow(nil)
	win.buf = b
	ws.Windows = append(ws.Windows, win)
	if ws.Root == nil {
		ws.Root = leafNode(win)
	} else {
		ws.Root = &WinNode{Dir: SplitVertical, Children: []*WinNode{ws.Root, leafNode(win)}}
	}
}

// WindowsForBuffer returns every window whose
// currently displayed buffer is buf. This is used to fan out mark-range
// adjustments (MarkAdjustInternal / MarkColAdjust) to every window that
// holds marks for the buffer being edited, since marks live per-window
// rather than per-buffer.
func (e *Editor) WindowsForBuffer(buf *Buffer) []*Window {
	var out []*Window
	for _, w := range e.Windows() {
		if w.buf == buf {
			out = append(out, w)
		}
	}
	return out
}

func (e *Editor) HandleInput(ev *tcell.EventKey) {
	var k *KeyHandler

	win := e.ActiveWindow()
	if win == nil {
		// Workspace teardown in progress (e.g. right after ":q"): drop event.
		return
	}
	buf := win.Buffer()
	if buf == nil {
		// No active buffer yet: drop event instead of nil-deref in buf.Mode().
		return
	}
	mode := buf.Mode()
	e.Message = ""

	if buf.KeyHandler != nil {
		k = buf.KeyHandler
	} else {
		k = e.Keys
	}

	if len(e.UiComponents) > 0 {
		comp := e.UiComponents[len(e.UiComponents)-1]
		k = comp.Keymap()
		mode = comp.Mode()
		// Share the main editor's macros manager so '.' records UI interactions too
		k.Macros = e.Keys.Macros
	}

	k.HandleKey(e, ev, mode)
}

func (e *Editor) LogError(err error, echo ...bool) {
	buf := e.BufferFindByFilePath("[Messages]", true)
	buf.Append("error: " + err.Error())
	if len(echo) > 0 && echo[0] == true {
		e.EchoMessage(err.Error())
	}
}

func (e *Editor) LogMessage(msg ...string) {
	for _, m := range msg {
		buf := e.BufferFindByFilePath("[Messages]", true)
		buf.Append(m)
	}
}

func (e *Editor) RuntimeDir(elems ...string) string {
	p := []string{os.Getenv("HOME"), ".config", "wig"}
	elems = append(p, elems...)
	return path.Join(elems...)
}

func (e *Editor) EchoMessage(msg string) {
	msg = strings.ReplaceAll(msg, "\n", " ")
	buf := e.BufferFindByFilePath("[Messages]", true)
	buf.Append(msg)
	e.Message = msg
}

// ClearYanks empties the yank history (registers). Used when loading a
// new session to prevent stale yanks referencing closed buffers.
func (e *Editor) ClearYanks() {
	e.Yanks = List[yank]{}
}

func (e *Editor) Redraw() {
	e.RedrawCh <- 1
}

func (e *Editor) ScreenSync() {
	e.ScreenSyncCh <- 1
}

// CmdToggleIndentGuides flips the IndentGuides config flag at runtime so
// you can show/hide virtual indent guides without restarting. The initial
// value comes from the "indent_guides" key in config.toml (default: true).
func CmdToggleIndentGuides(ctx Context) {
	ctx.Editor.Config.IndentGuides = !ctx.Editor.Config.IndentGuides
	ctx.Editor.Redraw()
}
