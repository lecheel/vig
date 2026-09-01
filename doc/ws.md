# Wig Workspace Design Specification

Based on a review of the codebase (`editor.go`, `workspace_cache.go`, `project.go`, `ui/statusline.go`, `ui/config_popup.go`, `cmd/main.go`), here is the design specification for the workspace management and session persistence system.

## 1. Overview

The workspace system allows users to manage multiple independent editing contexts simultaneously. Each workspace maintains its own set of windows, split-tree hierarchy (`WinNode`), layout orientation, open file history, and active buffer state.

The workspace system is deeply integrated with the `Editor` singleton and includes an on-disk persistence layer scoped to the active project root directory, enabling seamless per-project session save and restore across editor restarts.

## 2. Core Components

### 2.1. Workspace
A struct representing a single editing context.
- **`Num int`**: The 0-indexed identifier of the workspace (0–9).
- **`Windows []*Window`**: A flat slice of all window instances currently visible in this workspace.
- **`ActiveWindow *Window`**: The window currently receiving user input and focus.
- **`Root *WinNode`**: Root of the recursive split tree (`SplitVertical` or `SplitHorizontal`) managing visible window geometry.
- **`Layout Layout`**: Layout orientation mode (`LayoutVertical` or `LayoutHorizontal`).
- **`Files []string`**: The ordered history of every real file opened while this workspace was active (independent of visible window splits).

### 2.2. Editor (Workspaces Array)
The `Editor` struct holds a fixed-size array of workspaces:
- **`Workspaces []Workspace`**: Array of 10 workspace slots (indexes 0–9).
- **`ActiveWorkspace int`**: The index of the currently active workspace.
- *Initial State*: On startup, `NewEditor` initializes `Workspace[1]` with a single window, sets it as active, and sets `Root = leafNode(window)`. Workspace 0 is reserved.

### 2.3. WorkspaceCache (Per-Project Persistence)
Responsible for session persistence, keyed by the project's root directory:
- **Storage Location**: `~/.config/wig/workspaces/<sha256-project-hash>.json` (derived from `editor.Projects.GetRoot()`).
- **`ProjectRoot string`**: The canonical absolute path of the project associated with this cache.
- **`Workspaces map[int]WorkspaceCacheEntry`**: Maps workspace numbers to their cached state.
- **`ActiveWorkspace int`**: The workspace index that was active when the editor session ended.

### 2.4. WorkspaceCacheEntry
Represents the saved state of a single workspace:
- **`Windows []string`**: The real on-screen split layout: ordered slice of file paths corresponding 1:1 to visible windows.
- **`Files []string`**: The full open-file history for the workspace. Restored as background buffers without creating unwanted split windows.
- **`ActiveFile string`**: Absolute file path of the buffer focused when the session ended.
- **`Layout int`**: Saved split layout orientation (`0` = Horizontal, `1` = Vertical).

## 3. Configuration & Statusline Integration

### 3.1. User Configuration (`config.toml`)
Session persistence is controlled by the `save_workspaces` setting:
[editor]
save_workspaces = true # Enables auto-save & auto-restore per project
*(Also accepts aliases `workspaces = true` or `ws = true`)*

### 3.2. Statusline Indicators
Both plain and powerline statusline styles display the workspace state and persistence status on the right side:
- `💾 [ws:1]`: Workspace 1 active, **session persistence enabled** (`save_workspaces = true`).
- `🔒 [ws:1]`: Workspace 1 active, **session persistence disabled** (`save_workspaces = false`).

## 4. Lifecycle & State Management

### 4.1. Startup & Auto-Restore
When starting `wig` (`cmd/main.go`):
1. User config is loaded from `~/.config/wig/config.toml`.
2. If file arguments are passed (e.g. `wig main.go`), specific files are opened directly.
3. If no file arguments are passed and `editor.Config.SaveWorkspaces == true`:
   - `LoadWorkspaceCache(editor.Projects.GetRoot())` reads the project's cache file.
   - `wsCache.RestoreAll(editor)` restores all workspaces, their exact split layouts, background buffers, and focuses the previous active workspace and window.
4. If `editor.Config.SaveWorkspaces == false`, any previously saved state via `:wssave` is ignored. `ws1` will start as a blank, empty workspace.

### 4.2. Exit & Auto-Save
When the editor exits (`<-editor.ExitCh`):
1. If `editor.Config.SaveWorkspaces == true`:
   - `wsCache.CaptureAll(editor)` records visible windows, split layouts, and file histories for all workspaces.
   - `wsCache.Save(editor.Projects.GetRoot())` writes the state to `~/.config/wig/workspaces/<hash>.json`.

### 4.3. Switching Workspaces (`CmdWorkspaceSwitch_[0-9]`)
The `workspaceSwitch(ctx, num)` flow executes:
1. **Guard**: Ignores if `num` is out of bounds or already the active workspace.
2. **Capture**: Records the current workspace's state.
3. **Seed**: If the target workspace has no windows, initializes a default window.
4. **Activate**: Sets `editor.ActiveWorkspace = num`.
5. **Restore**: Calls `RestoreWorkspace` if the target workspace has not been populated in the current session.
6. **Redraw**: Requests an editor redraw.

### 4.4. Moving Windows Between Workspaces (`CmdWindowMoveToWorkspace_[0-9]`)
Moves the active window from the current workspace to another:
1. Adds the window/buffer to the target workspace's window list and split tree.
2. Closes the window in the current workspace via `removeLeaf` / `CmdWindowClose`.

## 5. Persistence Mechanics

### 5.1. Capturing State (`CaptureWorkspace` / `CaptureAll`)
- **Filtering**: Ignores scratch/system buffers (e.g., `[Messages]`, `[git]`, `[No Name]`) and deleted files.
- **Split Preservation**: `Windows` records the exact on-screen window order.
- **History Preservation**: `ws.Files` contains all files opened during the session, appended chronologically via `recordWorkspaceFile`.
- **Protection**: Empty/unmodified workspaces from prior sessions are preserved in the cache without being overwritten.

### 5.2. Restoring State (`RestoreWorkspace` / `RestoreAll`)
- **Split Rebuilding**: Recreates `Window` instances and builds the `WinNode` split tree from `entry.Windows`.
- **Background Buffers**: Loads non-visible files from `entry.Files` as background buffers via `editor.OpenFile` so they remain accessible in buffer pickers (MRU) without cluttering visible splits.
- **Window Focus**: Sets `ws.ActiveWindow` to the window matching `entry.ActiveFile`, falling back to the first window.
- **Missing File Grace**: If cached files were deleted from disk, `os.Stat` checks skip them safely, and `ensureWorkspaceBuffer` attaches a clean buffer to prevent nil-dereference panics.

## 6. Command Reference

| Command | Keybinding / Alias | Description |
| :--- | :--- | :--- |
| `CmdWorkspaceSwitch_0` .. `_9` | `<Leader>w0` .. `<Leader>w9` | Switch directly to workspace 0 through 9. |
| `CmdWorkspaceListPicker` | `<Leader>ww` | Open fuzzy picker listing all workspaces with open file summaries. |
| `ConfigPopupInit` | `:config` / UI | Interactive popup to toggle `save_workspaces` at runtime. |

## 7. Edge Cases Handled

1. **Multi-Project Isolation**: Workspaces for different repositories or directories do not conflict or overwrite each other.
2. **Buffer Deduplication**: Global buffers (`editor.BufferFindByFilePath`) are reused across workspaces without re-reading from disk.
3. **Buffer Destruction on `:q`**: Killing a buffer (`CmdKillBuffer`) safely substitutes a replacement across all workspaces and prunes it from `ws.Files` and `ws.Root`.
4. **Legacy Fallback**: If a project-specific workspace cache does not yet exist, `LoadWorkspaceCache` transparently checks legacy global files (`workspaces.json`) for seamless upgrades.
5. **Manual Save with Disabled Auto-Restore**: `ws1` is a special workspace. If `save_workspaces` is `false` in the config but the user manually saves using `:wssave`, the rest of the workspaces are written to the cache file. However, because auto-restore is disabled, `ws1` will ignore this on the next startup, and running `./wig` will result in an empty state (nothing loaded).
