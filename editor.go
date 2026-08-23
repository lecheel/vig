package wig

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
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

type Context struct {
	Editor *Editor
	Buf    *Buffer
	Win    *Window
	Count  uint32
	Char   string
}

type AutocompleteFn func(Context) bool

// MarksPopupFactory allows the `ui` package to register a popup for marks
// without causing a circular import.
var MarksPopupFactory func(ctx Context, marks map[rune]Cursor)

// RegistersPopupFactory allows the `ui` package to register a popup for registers
// without causing a circular import.
var RegistersPopupFactory func(ctx Context)

var EditorInst *Editor

type Layout int

const (
	LayoutHorizontal Layout = 0
	LayoutVertical   Layout = 1
)

type Editor struct {
	View                View
	Keys                *KeyHandler
	Buffers             []*Buffer
	Windows             []*Window
	activeWindow        *Window
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
	LastRepeatableFn    func(Context)
	ActiveRegister      rune
}

func NewEditor(
	view View,
	keys *KeyHandler,
) *Editor {
	windows := []*Window{CreateWindow(nil)}

	EditorInst = &Editor{
		View:         view,
		Keys:         keys,
		Buffers:      make([]*Buffer, 0, 32),
		Yanks:        List[yank]{},
		Windows:      windows,
		activeWindow: windows[0],
		Layout:       LayoutVertical,
		Projects:     NewProjectManager(),
		ExitCh:       make(chan int),
		RedrawCh:     make(chan int, 10),
		ScreenSyncCh: make(chan int),
		Events:       NewEventsManager(),
		Snippets:     NewSnippetsManager(),
	}

	EditorInst.Lsp = NewLspManager(EditorInst)
	TreeSitterHighlighterGo(EditorInst)

	return EditorInst
}

func (e *Editor) OpenFile(path string) (*Buffer, error) {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	if fbuf := e.BufferFindByFilePath(path, false); fbuf != nil {
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

	// Replace the initial empty "[No Name]" buffer if it's unmodified.
	// This prevents a useless [No Name] buffer from lingering in the
	// background after the user opens a real file.
	if len(e.Buffers) == 1 && e.Buffers[0].FilePath == "[No Name]" && !e.Buffers[0].Dirty {
		e.Buffers[0] = buf
		// Update any windows that were showing the old [No Name] buffer
		for _, win := range e.Windows {
			if win.buf != nil && win.buf.FilePath == "[No Name]" {
				win.ShowBuffer(buf)
			}
		}
	} else {
		e.Buffers = append(e.Buffers, buf)
	}

	e.Lsp.DidOpen(buf)

	hl := TreeSitterHighlighterInitBuffer(e, buf)
	if hl != nil {
		buf.Highlighter = hl
	}

	// Broadcast event so GitGutterManager calculates signs for newly opened file
	e.Events.Broadcast(EventBufferReloaded{Buf: buf})

	return buf, nil
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

// Returns active window buffer
func (e *Editor) ActiveBuffer() *Buffer {
	return e.ActiveWindow().Buffer()
}

func (e *Editor) ActiveWindow() *Window {
	return e.activeWindow
}

func (e *Editor) SetActiveWindow(w *Window) {
	e.activeWindow = w
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

func (e *Editor) EnsureBufferIsVisible(b *Buffer) {
	for _, win := range e.Windows {
		if win.Buffer() == b {
			return
		}
	}
	if len(e.Windows) > 1 {
		e.Windows[len(e.Windows)-1].ShowBuffer(b)
		return
	}
	win := CreateWindow(nil)
	win.buf = b
	e.Windows = append(e.Windows, win)
}

func (e *Editor) HandleInput(ev *tcell.EventKey) {
	var k *KeyHandler
	mode := e.ActiveBuffer().Mode()
	e.Message = ""

	if e.ActiveWindow().Buffer().KeyHandler != nil {
		k = e.ActiveWindow().Buffer().KeyHandler
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
