package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

// sessionWinNode is the JSON-serializable form of a wig.WinNode.
// Leaf nodes (Dir == SplitNone) carry the buffer's file path and the
// saved cursor position; split nodes carry the layout direction and
// recursive children.
type sessionWinNode struct {
	Dir          wig.SplitDir      `json:"dir"`
	FilePath     string            `json:"file_path,omitempty"`
	Line         int               `json:"line,omitempty"`
	Char         int               `json:"char,omitempty"`
	ScrollOffset int               `json:"scroll_offset,omitempty"`
	Active       bool              `json:"active,omitempty"`
	Children     []*sessionWinNode `json:"children,omitempty"`
}

// Session is a snapshot of one workspace's window layout, open files
// and cursor locations, written to ~/.config/wig/sessions/<name>.json
// by :mksession and restored by :session / :sessions.
type Session struct {
	Name      string          `json:"name"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
	Layout    wig.Layout      `json:"layout"`
	Files     []string        `json:"files"`
	Root      *sessionWinNode `json:"root"`
}

func sessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wig", "sessions")
}

func (s *Session) Save() error {
	dir := sessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, s.Name+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadSession(name string) (*Session, error) {
	path := filepath.Join(sessionsDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Name == "" {
		s.Name = name
	}
	return &s, nil
}

func ListSessions() ([]*Session, error) {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		s, err := LoadSession(name)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})
	return sessions, nil
}

func DeleteSession(name string) error {
	return os.Remove(filepath.Join(sessionsDir(), name+".json"))
}

// captureSessionNode walks the WinNode tree and produces a serializable
// snapshot. The active window is marked so restore can restore focus.
func captureSessionNode(node *wig.WinNode, activeWin *wig.Window) *sessionWinNode {
	if node == nil {
		return nil
	}
	if node.Dir == wig.SplitNone {
		out := &sessionWinNode{Dir: wig.SplitNone}
		win := node.Win
		if win != nil && win.Buffer() != nil {
			buf := win.Buffer()
			out.FilePath = buf.FilePath
			if cur := wig.WindowCursorGet(win, buf); cur != nil {
				out.Line = cur.Line
				out.Char = cur.Char
				out.ScrollOffset = cur.ScrollOffset
			}
			out.Active = win == activeWin
		}
		return out
	}
	out := &sessionWinNode{Dir: node.Dir}
	for _, c := range node.Children {
		if child := captureSessionNode(c, activeWin); child != nil {
			out.Children = append(out.Children, child)
		}
	}
	return out
}

// buildSessionTree reverses captureSessionNode: it materializes a
// WinNode tree from the saved snapshot, creating windows and loading
// files along the way. Newly created windows are appended to newWins.
// The active window (the leaf whose Active flag is set, or the first
// leaf if none) is returned via activeWin.
func buildSessionTree(
	ctx wig.Context,
	sn *sessionWinNode,
	newWins *[]*wig.Window,
	activeWin **wig.Window,
) *wig.WinNode {
	if sn == nil {
		return nil
	}
	if sn.Dir == wig.SplitNone {
		var buf *wig.Buffer
		if sn.FilePath != "" {
			if strings.HasPrefix(sn.FilePath, "[") {
				// Special buffers ([Messages], [No Name], ...) are looked
				// up by path instead of being re-read from disk.
				buf = ctx.Editor.BufferFindByFilePath(sn.FilePath, true)
			} else if b, err := ctx.Editor.OpenFile(sn.FilePath); err == nil {
				buf = b
			}
		}
		if buf == nil {
			buf = wig.NewBuffer()
			ctx.Editor.Buffers = append(ctx.Editor.Buffers, buf)
		}
		win := wig.CreateWindow(nil)
		win.ShowBuffer(buf)
		if cur := wig.WindowCursorGet(win, buf); cur != nil {
			line := sn.Line
			if line < 0 {
				line = 0
			}
			if buf.Lines.Len > 0 && line >= buf.Lines.Len {
				line = buf.Lines.Len - 1
			}
			if line < 0 {
				line = 0
			}
			cur.Line = line
			char := sn.Char
			if char < 0 {
				char = 0
			}
			cur.Char = char
			if sn.ScrollOffset > 0 {
				cur.ScrollOffset = sn.ScrollOffset
			} else {
				cur.ScrollOffset = 0
			}
		}
		*newWins = append(*newWins, win)
		if sn.Active || *activeWin == nil {
			*activeWin = win
		}
		return &wig.WinNode{Win: win}
	}
	var children []*wig.WinNode
	for _, c := range sn.Children {
		if child := buildSessionTree(ctx, c, newWins, activeWin); child != nil {
			children = append(children, child)
		}
	}
	if len(children) == 0 {
		return nil
	}
	if len(children) == 1 {
		return children[0]
	}
	return &wig.WinNode{Dir: sn.Dir, Children: children}
}

// CmdMakeSession saves the active workspace's layout, open files, and
// cursor positions to ~/.config/wig/sessions/<name>.json. With no
// argument the session is named after the workspace's last loaded
// session, the project root directory, or "default" if neither is known.
func CmdMakeSession(ctx wig.Context) {
	name := strings.TrimSpace(ctx.Char)
	ws := ctx.Editor.GetActiveWorkspace()
	if name == "" {
		if ws.ActiveSession != "" {
			name = ws.ActiveSession
		} else if root := ctx.Editor.Projects.GetRoot(); root != "" {
			name = filepath.Base(root)
		} else {
			name = "default"
		}
	}
	ws.ActiveSession = name

	activeWin := ctx.Editor.ActiveWindow()
	now := time.Now().Unix()
	session := &Session{
		Name:      name,
		UpdatedAt: now,
		Layout:    ws.Layout,
		Files:     append([]string{}, ws.Files...),
		Root:      captureSessionNode(ws.Root, activeWin),
	}
	if existing, err := LoadSession(name); err == nil && existing.CreatedAt > 0 {
		session.CreatedAt = existing.CreatedAt
	} else {
		session.CreatedAt = now
	}

	if err := session.Save(); err != nil {
		ctx.Editor.EchoMessage("mksession error: " + err.Error())
		return
	}

	count := 0
	for _, w := range ws.Windows {
		if w != nil && w.Buffer() != nil {
			count++
		}
	}
	ctx.Editor.EchoMessage(fmt.Sprintf("Session %q saved (%d windows, %d files)", name, count, len(session.Files)))
}

// CmdLoadSession restores a named session. With no argument, opens the
// session list popup instead.
func CmdLoadSession(ctx wig.Context) {
	name := strings.TrimSpace(ctx.Char)
	if name == "" {
		CmdSessionList(ctx)
		return
	}
	loadSessionByName(ctx, name)
}

func loadSessionByName(ctx wig.Context, name string) {
	session, err := LoadSession(name)
	if err != nil {
		ctx.Editor.EchoMessage("load session error: " + err.Error())
		return
	}
	if session.Root == nil {
		ctx.Editor.EchoMessage("session " + name + " is empty")
		return
	}

	// Pre-open every file in the session history so they appear in the
	// buffer picker even if no window shows them.
	for _, fp := range session.Files {
		if fp == "" || strings.HasPrefix(fp, "[") {
			continue
		}
		_, _ = ctx.Editor.OpenFile(fp)
	}

	var newWins []*wig.Window
	var activeWin *wig.Window
	newRoot := buildSessionTree(ctx, session.Root, &newWins, &activeWin)
	if newRoot == nil || len(newWins) == 0 {
		ctx.Editor.EchoMessage("session " + name + " has no windows")
		return
	}

	ws := ctx.Editor.GetActiveWorkspace()
	ws.Windows = newWins
	ws.Root = newRoot
	ws.Layout = session.Layout
	ws.ActiveSession = name
	if activeWin != nil {
		ws.ActiveWindow = activeWin
	} else {
		ws.ActiveWindow = newWins[0]
	}
	ws.Files = append([]string{}, session.Files...)

	ctx.Editor.Redraw()
	ctx.Editor.ScreenSync()
	ctx.Editor.EchoMessage(fmt.Sprintf("Session %q loaded (%d windows)", name, len(newWins)))
}

// CmdSessionList opens a popup listing every saved session. Enter
// loads the selected session; if no session is selected but the input
// is non-empty, Enter saves the current workspace as a new session
// with that name. Delete removes the selected session.
func CmdSessionList(ctx wig.Context) {
	refreshItems := func() []ui.PickerItem[string] {
		sessions, _ := ListSessions()
		items := make([]ui.PickerItem[string], 0, len(sessions))
		for _, s := range sessions {
			ts := "--/-- --:--"
			if s.UpdatedAt > 0 {
				ts = time.Unix(s.UpdatedAt, 0).Format("01-02 15:04")
			}
			items = append(items, ui.PickerItem[string]{
				Name:  fmt.Sprintf("[%s] %s", ts, s.Name),
				Value: s.Name,
			})
		}
		return items
	}

	action := func(p *ui.UiPicker[string], i *ui.PickerItem[string]) {
		defer ctx.Editor.PopUi()
		if i == nil {
			// No session selected — save current as a new session named
			// by the picker's input (skipped if input is empty).
			name := strings.TrimSpace(p.GetInput())
			if name == "" {
				return
			}
			origChar := ctx.Char
			ctx.Char = name
			CmdMakeSession(ctx)
			ctx.Char = origChar
			return
		}
		loadSessionByName(ctx, i.Value)
	}

	picker := ui.PickerInit(ctx.Editor, action, refreshItems())
	picker.SetTitle("Sessions [Enter: Load/Save] [Delete: Remove] [Esc: Close]")

	picker.OnKey("Delete", func(ctx wig.Context) {
		item := picker.GetActiveItem()
		if item == nil {
			return
		}
		_ = DeleteSession(item.Value)
		picker.SetItems(refreshItems())
		ctx.Editor.Redraw()
	})
}

// CmdSessionToggle opens the session list popup if no UI component is
// currently on top, or pops the topmost component otherwise. Bound to
// a single key (e.g. <leader>S) this gives "toggle" behavior: pressing
// the key once shows the list, pressing it again hides it.
func CmdSessionToggle(ctx wig.Context) {
	if len(ctx.Editor.UiComponents) > 0 {
		ctx.Editor.PopUi()
		return
	}
	CmdSessionList(ctx)
}
