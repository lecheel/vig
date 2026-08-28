# Wig Workspace Design Specification

Based on a review of the codebase (`editor.go`, `workspace_cache.go`, `movements_cmds.go`, `commands/commands.go`), here is the design specification for the workspace management system.

## 1. Overview

The workspace system allows users to manage multiple independent editing contexts simultaneously. Each workspace maintains its own set of windows and active buffer state, enabling rapid context switching between different tasks (e.g., frontend vs. backend, editing vs. debugging).

The workspace system is deeply integrated with the `Editor` singleton and includes an on-disk persistence layer to restore sessions across editor restarts.

## 2. Core Components

### 2.1. Workspace
A struct representing a single editing context.
- **`Num int`**: The 0-indexed identifier of the workspace.
- **`Windows []*Window`**: A list of all windows currently open in this workspace.
- **`ActiveWindow *Window`**: The window currently receiving input and focus.

### 2.2. Editor (Workspaces Array)
The `Editor` struct holds a fixed-size array of workspaces:
- **`Workspaces []Workspace`**: Currently hardcoded to a length of 10 (indexes 0-9).
- **`ActiveWorkspace int`**: The index of the currently active workspace.
- *Initial State*: On startup, `NewEditor` initializes `Workspace[1]` with a single window and sets it as active. Workspace 0 is left empty initially.

### 2.3. WorkspaceCache (Persistence)
A struct stored on disk at `~/.config/wig/workspaces.json` responsible for session persistence.
- **`Workspaces map[int]WorkspaceCacheEntry`**: Maps workspace numbers to their cached state.
- **`ActiveWorkspace int`**: The workspace that was active when the editor was closed.

### 2.4. WorkspaceCacheEntry
Represents the saved state of a single workspace.
- **`Files []string`**: A deduplicated list of absolute file paths open in the workspace.
- **`ActiveFile string`**: The file path of the buffer that was active in the workspace.

## 3. Lifecycle & State Management

### 3.1. Initialization
When the editor starts without file arguments (`cmd/main.go`):
1. `LoadWorkspaceCache()` is called.
2. If `ActiveWorkspace` from the cache has files associated with it, `RestoreWorkspace` is called.
3. If the target workspace has no windows (fresh start), a default window is created.

### 3.2. Switching Workspaces
Triggered via `CmdWorkspaceSwitch_[0-9]` or the workspace picker (`CmdWorkspaceListPicker`).
The `workspaceSwitch(ctx, num)` function executes the following sequence:
1. **Guard**: Ignore if `num` is the current `ActiveWorkspace` or out of bounds.
2. **Capture**: Record the current workspace's open files into the `WorkspaceCache` and save to disk.
3. **Seed**: If the target workspace has no windows, create a default window.
4. **Activate**: Set `editor.ActiveWorkspace = num`.
5. **Restore**: Call `RestoreWorkspace` to open cached files if the target workspace is currently empty.
6. **Render**: Trigger an editor redraw.

### 3.3. Moving Windows Between Workspaces
Triggered via `CmdWindowMoveToWorkspace_[0-9]`.
The `moveWindowToWorkspace(ctx, num)` function:
1. Creates a new window in the target workspace cloned from the active window.
2. Closes the window in the current workspace using `CmdWindowClose`.

## 4. Persistence Logic

### 4.1. Capturing State (`CaptureWorkspace`)
Iterates through all windows in a given workspace to build a `WorkspaceCacheEntry`.
- **Filtering**: Skips windows with nil buffers, special buffers (e.g., `[Messages]`, `[git]`), or file paths that no longer exist on disk.
- **Deduplication**: Uses a `seen` map to ensure files appearing in multiple splits are only listed once.
- **Protection**: If the workspace ends up with zero file-backed buffers (e.g., only scratch buffers), the cache entry is *not* overwritten. This prevents silently erasing a previous session's file list just because the user opened a blank workspace.

### 4.2. Restoring State (`RestoreWorkspace`)
Rebuilds the window layout of a workspace from a `WorkspaceCacheEntry`.
- **Guard**: Only restores if the workspace currently has no real file-backed buffers (prevents duplicating files on a "warm" workspace).
- **File Validation**: Checks `os.Stat(fp)` before opening. Deleted files are skipped gracefully.
- **Window Creation**: Opens the file via `editor.OpenFile`, creates a new window, and attaches the buffer. Every cached file gets its own window.
- **Focus**: Sets `ws.ActiveWindow` to the window holding `entry.ActiveFile`. Falls back to the first surviving window if the active file was deleted.

## 5. UI & Interaction

### 5.1. Statusline Integration
The `ui/statusline.go` module displays the current workspace index on the right side of the status bar:
`[workspace: %d] %d:%d`

### 5.2. Workspace Picker (`CmdWorkspaceListPicker`)
Provides a fuzzy-finding interface for workspace management.
- **Display**: Lists all 10 workspaces. Appends ` *` to the active one. Shows a list of base filenames for each workspace, or `(empty)`.
- **Switching (Enter)**: Captures current workspace state, switches to the selected workspace, and restores it.
- **Clearing (Delete)**: Explicitly removes a workspace's entry from `WorkspaceCache` and saves the cache. This is the only way to permanently clear a persisted workspace.

## 6. API Surface (Commands)

| Command | Description |
| :--- | :--- |
| `CmdWorkspaceSwitch_[0-9]` | Instantly switches to the corresponding workspace index. |
| `CmdWindowMoveToWorkspace_[0-9]` | Moves the currently active window to the target workspace. |
| `CmdWorkspaceListPicker` (`wslist`) | Opens the workspace management fuzzy finder. |

## 7. Design Constraints & Edge Cases Handled

1. **Nil Buffers on Teardown**: If `CmdWindowClose` removes the last window, `HandleInput` and `ActiveBuffer()` gracefully handle nil buffers instead of panicking.
2. **Deleted Files**: Both `CaptureWorkspace` and `RestoreWorkspace` verify file existence on disk (`os.Stat`). Deleted files are dropped from the cache or skipped during restore.
3. **Empty Workspaces**: If a workspace is restored but all its cached files have been deleted, `ensureWorkspaceBuffer` attaches a fresh empty buffer to the active window to prevent a nil-pointer panic.
4. **Buffer Deduplication**: `editor.BufferFindByFilePath` ensures that opening a file that is already open globally does not create a duplicate buffer; it simply brings it into the workspace's window.
