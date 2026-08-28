package wig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceCacheEntry stores the file paths and active file for a single
// workspace so that the workspace layout can be restored in a later
// editor session.
type WorkspaceCacheEntry struct {
	// Windows is the real on-screen split layout: one file path per
	// Window that existed in the workspace, in window order. This is
	// what RestoreWorkspace uses to recreate splits — never Files.
	Windows []string `json:"windows"`
	// Files is every file ever opened in this workspace (see
	// Editor.recordWorkspaceFile), independent of how many windows
	// existed. Restored as background buffers only, no windows.
	Files      []string `json:"files"`
	ActiveFile string   `json:"active_file"`
	Layout     int      `json:"layout"`
}

// WorkspaceCache is the on-disk persistence store for workspace state.
// It is saved to ~/.config/wig/workspaces.json on exit and loaded on
// startup or when the workspace picker is opened.
type WorkspaceCache struct {
	Workspaces      map[int]WorkspaceCacheEntry `json:"workspaces"`
	ActiveWorkspace int                         `json:"active_workspace"`
}

// LoadWorkspaceCache reads the workspace cache from disk. If the file
// does not exist or is corrupt, an empty cache is returned.
func LoadWorkspaceCache() *WorkspaceCache {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".config", "wig", "workspaces.json")

	data, err := os.ReadFile(p)
	if err != nil {
		return &WorkspaceCache{Workspaces: make(map[int]WorkspaceCacheEntry)}
	}

	var cache WorkspaceCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return &WorkspaceCache{Workspaces: make(map[int]WorkspaceCacheEntry)}
	}
	if cache.Workspaces == nil {
		cache.Workspaces = make(map[int]WorkspaceCacheEntry)
	}
	return &cache
}

// Save writes the workspace cache to disk.
func (c *WorkspaceCache) Save() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "wig")
	os.MkdirAll(dir, 0755)
	p := filepath.Join(dir, "workspaces.json")
	data, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(p, data, 0644)
}

// CaptureWorkspace records the file paths open in a workspace into the
// cache. Only real file-backed buffers are stored; special buffers
// like [Messages] or [git] are skipped. Duplicate paths are deduplicated.
func (c *WorkspaceCache) CaptureWorkspace(num int, ws *Workspace) {
	if ws == nil {
		return
	}
	entry := WorkspaceCacheEntry{}
	exists := func(fp string) bool {
		if fp == "" || strings.HasPrefix(fp, "[") {
			return false
		}
		// Drop files that no longer exist on disk so deleted files never
		// poison future session restores.
		_, err := os.Stat(fp)
		return err == nil
	}
	// Windows records the REAL split layout: exactly one entry per
	// window currently on screen, in order. This — not Files — is what
	// RestoreWorkspace uses to decide how many windows to recreate, so
	// a workspace with 10 buffers opened serially in one window still
	// restores as one window, not ten splits.
	for _, win := range ws.Windows {
		if win == nil {
			continue
		}
		buf := win.Buffer()
		if buf == nil || !exists(buf.FilePath) {
			continue
		}
		entry.Windows = append(entry.Windows, buf.FilePath)
	}
	// Files is the full open-file history for the workspace (see
	// Editor.recordWorkspaceFile) — every file ever opened while this
	// workspace was active, regardless of how many windows existed.
	// Restored as background buffers only (no window each), so opening
	// file1 -> file2 -> file3 in the same window still leaves all three
	// reachable via the buffer picker/MRU after a restart.
	seen := make(map[string]bool)
	for _, fp := range ws.Files {
		if !exists(fp) || seen[fp] {
			continue
		}
		seen[fp] = true
		entry.Files = append(entry.Files, fp)
	}
	if ws.ActiveWindow != nil {
		buf := ws.ActiveWindow.Buffer()
		if buf != nil {
			entry.ActiveFile = buf.FilePath
		}
	}
	entry.Layout = int(ws.Layout)
	c.Workspaces[num] = entry
}

// CaptureAll captures the state of every workspace that has at least
// one window. Workspaces that were never used in this session are
// skipped so that their cached entries from a previous session are
// preserved.
func (c *WorkspaceCache) CaptureAll(editor *Editor) {
	for i := range editor.Workspaces {
		ws := &editor.Workspaces[i]
		if len(ws.Windows) == 0 {
			continue
		}
		c.CaptureWorkspace(i, ws)
	}
	c.ActiveWorkspace = editor.ActiveWorkspace
}

// RestoreWorkspace rebuilds the target workspace from cache if it is
// currently empty (no real file-backed buffers): every cached file gets
// its own window so the previous split layout is recreated. If the
// workspace already has file buffers, the cache is skipped to avoid
// duplicates.
func (c *WorkspaceCache) RestoreWorkspace(editor *Editor, num int) {
	entry, ok := c.Workspaces[num]
	if !ok || (len(entry.Windows) == 0 && len(entry.Files) == 0) {
		c.ensureWorkspaceBuffer(editor, num)
		return
	}
	ws := editor.GetWorkspace(num)
	// Skip if workspace already has real file buffers
	hasFiles := false
	for _, win := range ws.Windows {
		if win == nil {
			continue
		}
		buf := win.Buffer()
		if buf != nil && buf.FilePath != "" && !strings.HasPrefix(buf.FilePath, "[") {
			hasFiles = true
			break
		}
	}
	if hasFiles {
		return
	}
	// Recreate one window per entry in entry.Windows — this is the real
	// split layout the user had, independent of how many files were ever
	// opened. entry.Files is restored separately below as plain
	// background buffers (no window each), so a workspace with 10 files
	// opened serially in one window does NOT explode into 10 splits.
	windowFiles := entry.Windows
	if len(windowFiles) == 0 {
		// Legacy cache entries (pre-Windows field) or a workspace that
		// was never captured with real window info: fall back to a
		// single window on the most recently active file.
		if entry.ActiveFile != "" {
			windowFiles = []string{entry.ActiveFile}
		} else if len(entry.Files) > 0 {
			windowFiles = []string{entry.Files[len(entry.Files)-1]}
		}
	}
	type restoredWin struct {
		fp  string
		win *Window
	}
	var restored []restoredWin
	for _, fp := range windowFiles {
		// Files can vanish from disk between capture and restore. Skip
		// them instead of ending up with a nil-buffer window.
		if _, err := os.Stat(fp); err != nil {
			continue
		}
		buf, _ := editor.OpenFile(fp)
		if buf == nil {
			continue
		}
		win := CreateWindow(nil)
		ctx := editor.NewContext()
		ctx.Buf = buf
		win.VisitBuffer(ctx)
		restored = append(restored, restoredWin{fp: fp, win: win})
	}
	if len(restored) == 0 {
		// Every cached window-file was deleted from disk: still give the
		// workspace's active window a valid buffer.
		c.ensureWorkspaceBuffer(editor, num)
	} else {
		// If the workspace has a single window showing an unmodified
		// [No Name], replace its buffer with the first restored file
		// instead of orphaning it.
		if len(ws.Windows) == 1 {
			win := ws.Windows[0]
			if win != nil && win.Buffer() != nil && win.Buffer().FilePath == "[No Name]" && !win.Buffer().Dirty {
				ctx := editor.NewContext()
				ctx.Buf = restored[0].win.Buffer()
				win.VisitBuffer(ctx)
				restored[0].win = win
			}
		}
		ws.Windows = make([]*Window, 0, len(restored))
		for _, r := range restored {
			ws.Windows = append(ws.Windows, r.win)
		}
		ws.Layout = Layout(entry.Layout)
		if len(ws.Windows) == 1 {
			ws.Root = leafNode(ws.Windows[0])
		} else if len(ws.Windows) > 1 {
			splitDir := SplitVertical
			if ws.Layout == LayoutHorizontal {
				splitDir = SplitHorizontal
			}
			children := make([]*WinNode, len(ws.Windows))
			for i, win := range ws.Windows {
				children[i] = leafNode(win)
			}
			ws.Root = &WinNode{Dir: splitDir, Children: children}
		}
		// Focus the window showing the previously active file; fall back
		// to the first surviving window.
		ws.ActiveWindow = restored[0].win
		for _, r := range restored {
			if r.fp == entry.ActiveFile {
				ws.ActiveWindow = r.win
				break
			}
		}
	}
	// Restore the rest of the open-file history as background buffers
	// only — no windows — so they're reachable via the buffer picker/MRU
	// without forcing any layout.
	windowSet := make(map[string]bool, len(windowFiles))
	for _, fp := range windowFiles {
		windowSet[fp] = true
	}
	for _, fp := range entry.Files {
		if windowSet[fp] {
			continue
		}
		if _, err := os.Stat(fp); err != nil {
			continue
		}
		editor.OpenFile(fp)
	}
}

// ensureWorkspaceBuffer attaches a fresh empty buffer to the workspace's
// active window if it currently has none. Without this, restoring a
// workspace whose cached files were deleted from disk leaves a window with
// a nil buffer, which panics on the next input or render tick.
func (c *WorkspaceCache) ensureWorkspaceBuffer(editor *Editor, num int) {
	ws := editor.GetWorkspace(num)
	if ws.ActiveWindow == nil || ws.ActiveWindow.Buffer() != nil {
		return
	}
	ctx := editor.NewContext()
	ctx.Buf = NewBuffer()
	ws.ActiveWindow.VisitBuffer(ctx)
	if ws.Root == nil {
		ws.Root = leafNode(ws.ActiveWindow)
	}
}
