package wig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// MockView implements the View interface for testing purposes
type MockView struct{}

func (m *MockView) SetContent(x, y int, str string, st tcell.Style) {}
func (m *MockView) Size() (int, int)                                { return 80, 24 }
func (m *MockView) Resize(x, y, width, height int)                  {}

// setupTestEditor initializes an editor with a temporary HOME directory
// to prevent overwriting real user configuration/cache files.
func setupTestEditor(t *testing.T) (*Editor, string) {
	t.Helper()

	// Override HOME so workspace_cache.go writes to a temp directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// We also need a temporary project root to act as the working directory
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	keys := NewKeyHandler(ModeKeyMap{})
	editor := NewEditor(&MockView{}, keys)

	// Force the ProjectManager to use our temp rootDir
	editor.Projects.root = rootDir

	return editor, rootDir
}

// TestCmdKillBufferWorkspaceIsolation ensures that killing the last buffer
// in one workspace makes it fall back to [No Name] instead of stealing
// a buffer from another workspace.
func TestCmdKillBufferWorkspaceIsolation(t *testing.T) {
	editor, rootDir := setupTestEditor(t)

	// 1. Setup WS1 with file A
	fileA := filepath.Join(rootDir, "file_a.go")
	os.WriteFile(fileA, []byte("package a\n"), 0644)

	editor.ActiveWorkspace = 1
	_, err := editor.OpenFile(fileA)
	if err != nil {
		t.Fatalf("Failed to open file A: %v", err)
	}

	// 2. Setup WS3 with file B
	editor.ActiveWorkspace = 3
	ws3 := editor.GetWorkspace(3)

	// Ensure WS3 has an active window before opening files
	if len(ws3.Windows) == 0 {
		win := CreateWindow(nil)
		ws3.Windows = []*Window{win}
		ws3.ActiveWindow = win
		ws3.Root = leafNode(win)
	}

	// Give WS3 a [No Name] buffer so OpenFile replaces it naturally
	nb := NewBuffer()
	nb.FilePath = "[No Name]"
	editor.Buffers = append(editor.Buffers, nb)
	ws3.ActiveWindow.ShowBuffer(nb)

	fileB := filepath.Join(rootDir, "file_b.go")
	os.WriteFile(fileB, []byte("package b\n"), 0644)

	bufB, err := editor.OpenFile(fileB)
	if err != nil {
		t.Fatalf("Failed to open file B: %v", err)
	}

	// 3. Execute CmdKillBuffer on WS3's file
	ctx := Context{
		Editor: editor,
		Buf:    bufB,
		Win:    ws3.ActiveWindow,
	}
	CmdKillBuffer(ctx)

	// 4. Assertions

	// WS3 should now have a [No Name] buffer, NOT file_a.go
	ws3After := editor.GetWorkspace(3)
	if ws3After.ActiveWindow.Buffer().FilePath != "[No Name]" {
		t.Errorf("Expected WS3 to fall back to [No Name], got %s", ws3After.ActiveWindow.Buffer().FilePath)
	}

	// WS3 files history should be empty
	if len(ws3After.Files) != 0 {
		t.Errorf("Expected WS3 files history to be empty, got %d items", len(ws3After.Files))
	}

	// WS1 should be unaffected and still hold file A
	ws1After := editor.GetWorkspace(1)
	if ws1After.ActiveWindow.Buffer().FilePath != fileA {
		t.Errorf("Expected WS1 to still have file A, got %s", ws1After.ActiveWindow.Buffer().FilePath)
	}

	// WS1 files history should still have 1 item
	if len(ws1After.Files) != 1 {
		t.Errorf("Expected WS1 files history to have 1 file, got %d items", len(ws1After.Files))
	}
}

// TestWorkspaceCacheStaleCleanup ensures that emptying a workspace
// removes its entry from the cache on save, preventing ghost files
// from reappearing on restart.
func TestWorkspaceCacheStaleCleanup(t *testing.T) {
	editor, rootDir := setupTestEditor(t)

	// 1. Create files and open them in different workspaces
	fileA := filepath.Join(rootDir, "file_a.go")
	os.WriteFile(fileA, []byte("a\n"), 0644)
	editor.ActiveWorkspace = 1
	editor.OpenFile(fileA)

	fileB := filepath.Join(rootDir, "file_b.go")
	os.WriteFile(fileB, []byte("b\n"), 0644)
	editor.ActiveWorkspace = 2

	// Manually setup WS2 window
	ws2 := editor.GetWorkspace(2)
	win := CreateWindow(nil)
	ws2.Windows = []*Window{win}
	ws2.ActiveWindow = win
	ws2.Root = leafNode(win)
	editor.OpenFile(fileB)

	// 2. Capture state
	cache := LoadWorkspaceCache(rootDir)
	cache.CaptureAll(editor)

	if _, ok := cache.Workspaces[1]; !ok {
		t.Error("Expected WS1 to be in cache initially")
	}
	if _, ok := cache.Workspaces[2]; !ok {
		t.Error("Expected WS2 to be in cache initially")
	}

	// 3. Simulate killing the buffer in WS2
	// This removes the file from ws.Files and sets the buffer to [No Name]
	ws2.Files = []string{}
	ws2.Windows[0].buf = NewBuffer()

	// 4. Capture state again
	cache.CaptureAll(editor)

	// 5. Assertions

	// WS2 should now be removed from the cache entirely because it's empty
	if _, ok := cache.Workspaces[2]; ok {
		t.Errorf("Expected WS2 to be cleared from cache, but it is still present")
	}

	// WS1 should still be safely in the cache
	if _, ok := cache.Workspaces[1]; !ok {
		t.Errorf("Expected WS1 to still be in cache, but it was removed")
	}
}

// TestWorkspaceCacheCaptureAndRestore ensures that saving and loading
// the cache correctly rebuilds the workspace layout and history.
func TestWorkspaceCacheCaptureAndRestore(t *testing.T) {
	editor, rootDir := setupTestEditor(t)

	// 1. Setup WS1 with file A
	fileA := filepath.Join(rootDir, "file_c.go")
	os.WriteFile(fileA, []byte("c\n"), 0644)
	editor.ActiveWorkspace = 1
	editor.OpenFile(fileA)

	// 2. Capture and Save
	cache := LoadWorkspaceCache(rootDir)
	cache.CaptureAll(editor)
	cache.Save()

	// 3. Create a fresh editor to simulate a restart
	keys := NewKeyHandler(ModeKeyMap{})
	newEditor := NewEditor(&MockView{}, keys)
	newEditor.Projects.root = rootDir

	// 4. Load the cache
	loadedCache := LoadWorkspaceCache(rootDir)
	loadedCache.RestoreAll(newEditor)

	// 5. Assertions
	ws1 := newEditor.GetWorkspace(1)

	// Should have restored the file into the history
	if len(ws1.Files) != 1 || ws1.Files[0] != fileA {
		t.Errorf("Expected WS1 to restore file history with %s, got %v", fileA, ws1.Files)
	}

	// Should have created a window and set the buffer
	if ws1.ActiveWindow == nil || ws1.ActiveWindow.Buffer() == nil {
		t.Fatal("Expected WS1 to have an active window with a buffer")
	}

	if ws1.ActiveWindow.Buffer().FilePath != fileA {
		t.Errorf("Expected WS1 active buffer to be %s, got %s", fileA, ws1.ActiveWindow.Buffer().FilePath)
	}
}
