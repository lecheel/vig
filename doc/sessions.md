# Workspace Sessions

`wig` supports saving and restoring workspace sessions, including window splits, open files, cursor positions, and scroll offsets. This is similar to Vim's `:mksession` feature.

## Usage

### Commands
- **`:mks[ession] [name] [remark...]`**: Saves the current workspace layout to `~/.config/wig/sessions/<name>.json`.
  - If no name is provided, it uses the active session name, the project root directory name, or `"default"`.
  - If a name is provided and already exists, a `y/n` prompt will appear on the statusline to confirm overwriting.
  - You can append a remark or description after the session name (e.g., `:mks 01 test case for login bug`). If overwriting an existing session without providing a new remark, the existing remark is preserved.
- **`:session [name]`**: Loads a saved session. If no name is provided, opens the Session List popup.
- **`:sessions`** or **`:sl`**: Opens the Session List popup.

### Keybindings
- **`<leader>S`** (Leader + Shift+s): Toggles the Session List popup.

### Session List Popup
- **`Enter`**: Loads the highlighted session. If no session is highlighted and a new name is typed, it saves the current workspace as a new session with that name.
- **`Delete`**: Removes the highlighted session file from disk.
- **`Esc` / `q`**: Closes the popup.

## Auto Session (Restore on Startup)

`wig` can automatically restore your last active session when you start the editor without passing any file arguments.

### Configuration
To enable this, add `auto_session = true` to your `~/.config/wig/config.toml` under the `[editor]` section:

[editor]
auto_session = true

When enabled, the editor will:
1. On startup: Load the session tracked in `~/.config/wig/sessions/last_session`.
2. On quit: Save the active session (only if no buffers have unsaved changes).

## Implementation Details

### Storage
Sessions are stored as JSON files in `~/.config/wig/sessions/`.
- `<name>.json`: The session layout, files, and cursor states.
- `last_session`: A plain text file containing the name of the last loaded/saved session. This prevents "blind" auto-loading by ensuring the editor explicitly tracks which session was active, rather than guessing from file modification times.

### Data Structures
- `Session`: The serializable representation of a workspace. Includes `Name`, `Remark`, `CreatedAt`, `UpdatedAt`, `Layout`, `Files`, and `Root`.
- `sessionWinNode`: The serializable representation of a `WinNode` (window split tree).
- `Workspace.ActiveSession`: Added to track the active session name per workspace.

### Data Loss Prevention
To prevent losing unsaved buffer contents:
- Before saving, loading, or managing sessions, the editor checks if any buffer is dirty (`b.Dirty`).
- If dirty buffers are found, the operation is aborted, and an error message is displayed in the echo area.
- The auto-save on exit is skipped if any buffers have unsaved changes.

### Statusline Integration
The active session name is displayed on the right side of the statusline, following the powerline design pattern.
- The `StatuslineData` struct includes `SessionName` and `AutoSession` fields.
- `extractStatuslineData` populates these from `e.GetActiveWorkspace().ActiveSession` and `e.Config.AutoSession`.
- `renderPlain` and `renderPowerline` render it appropriately on the right side.
- An icon is displayed next to the session name to indicate the save behavior:
  - `` indicates `auto_session = true` (the session will automatically save on exit).
  - `✔` indicates `auto_session = false` (the session requires a manual `:mks` to save).
- `global_statusline = true` can be set in `config.toml` to render a single statusline at the bottom of the screen instead of one per split window.

### Overwrite Prompt & Remarks
When executing `:mksession <name> [remark]` and `<name>` already exists, the editor uses `ui.ConfirmInit` to display a minimal `y/n` prompt on the statusline, avoiding the heavier picker popup for a simple yes/no decision. If overwriting and no new remark is provided, the existing session's remark is preserved.

## Files Modified
- `cmd/main.go`: Auto-load on startup and auto-save on exit.
- `commands/sessions.go`: Core session logic, JSON serialization, tree capture/restoration, dirty checks.
- `commands/definitions.go`: Registration of `:mksession`, `:session`, etc.
- `config/config.go`: Added `auto_session` and `global_statusline` to `EditorSettings`.
- `editor.go`: Added `AutoSession` and `GlobalStatusline` to `EditorConfig`, and `ActiveSession` to `Workspace`.
- `render/render.go`: Added support for `global_statusline` rendering.
- `ui/config_popup.go`: Added `auto_session` and `global_statusline` toggles to the config popup.
- `ui/statusline.go`: Added session name rendering, `AutoSession` icons, and `GlobalStatusline` support.
