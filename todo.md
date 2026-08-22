# Wig - TODO & Roadmap

## ✅ Currently Implemented

- **Modes**
  - Normal, Insert, Visual
  - Visual Line, Visual Block

- **Core Editing**
  - Text insertion and deletion
  - Myers diff-based Undo / Redo
  - Yank and Put with rectangle block support
  - Auto-indentation and comment toggling (`CmdToggleComment`)

- **Movement**
  - Directional navigation: `h`, `j`, `k`, `l`, `w`, `b`, `0`, `$`
  - Target jumps: `%` (matching pair), `f`/`F`/`t`/`T` (find before/to character)
  - Scrolling: Page Up/Down, Line Up/Down
  - EasyMotion: `s` 2-character viewport jump with home-row target overlays (Normal and Visual)

- **Marks**
  - `m[a-z]` to set local buffer marks
  - `` ` `` popup with mark legends and line preview hints
  - `` `[a-z] `` to jump to mark, ``` `` ``` for ping-pong jump toggle
  - Visual mark indicators displayed in the line number gutter (yellow)

- **Search**
  - Incremental search (`/`) with live match highlighting and cursor preview
  - Result navigation with `n` (next) and `N` (previous)
  - Project-wide search via ripgrep (`<Leader>f`, `:s`)
  - Word under cursor search (`CmdSearchWordUnderCursor`, `CmdRgUnderCursor`)

- **File Management**
  - Open and save files (with auto-creation when path does not exist)
  - Buffer picker (`CmdBufferPicker`)
  - Most Recently Used buffer picker (`CmdMRUBufferPicker`)
  - File picker and command palette

- **Window Management**
  - Vertical split (`:vs`, `CmdWindowVSplit`)
  - Window cycling (`ctrl+w w`, `CmdWindowNext`)
  - Window closing (`:close`, `:only`, close & kill buffer)
  - Layout toggling between horizontal and vertical layouts

- **Git Integration**
  - Git status panel (`gs`/`CmdGitView`): staging (`s`), diff inspection (`d`), refresh (`r`)
  - Interactive commit editor: `c` opens commit buffer, `ctrl+c` finishes, `Esc`/`q` cancels
  - Remote push: `p` prompts with branch confirmation before pushing to `origin`
  - Stash management: stash unstaged changes (`z`), `Enter` on stash item for Pop/Drop/Cancel prompt
  - Stash diff preview (`d` on stash entry)
  - Fast section jumping: `l`/`L` moves to first item in next/previous section
  - Dynamic Diff & Commit highlighting (`DiffHighlighter`) using active theme colors
  - Tree-sitter query definitions for `diff`, `git`, and `gitcommit` (`queries/*/highlights.scm`)
  - Git gutter signs (+, -, ~) and hunk navigation (`]h`/`[h`), preview, and revert
  - Inline Git Blame (`blame`) and commit inspection buffer (`CmdGitBlameCommit`)
  - Clean buffer lifecycle: `q`/`Esc` kills temporary git, diff, and commit buffers

- **LSP (Language Server Protocol)**
  - Autocompletion popup with auto-trigger
  - Go to definition (`gd`, `CmdGotoDefinition`, `CmdGotoDefinitionOtherWindow`)
  - Hover documentation popup (`K`, `CmdLspHover`)
  - Signature help (`CmdLspShowSignature`)
  - Document diagnostics and Quickfix list (`:copen`)

- **Syntax Highlighting**
  - Tree-sitter integration for Go, Rust, Odin, C, and Python
  - Custom query support via `~/.config/wig/queries/<lang>/highlights.scm`

- **UI Elements**
  - Statusline with mode, file status, and cursor coordinates
  - Notification toasts (`ui.Notify` with 5s auto-dismiss, stacking, and type badges)
  - Which-Key popup (multi-column, mode title, configurable `words`/`camel`/`cmd` formats)
  - EasyMotion visual jump labels overlay
  - Command line, search prompt, hover popup, and autocomplete popup
  - Confirm prompts (`ui.ConfirmInit`) and marks popup (`ui.mark`)
  - Unified popup border and title styling (`ui.popup.title`)

- **System & Integration**
  - System clipboard: copy (`<Leader>y`), paste at cursor (`<Leader>pv`), replace buffer (`<Leader>pp`)
  - Bracketed paste mode support (`tcell.EventPaste`)
  - Cross-platform clipboard health checks (`checkhealth`: `pbcopy`, `wl-clipboard`, `xclip`, `xsel`, Windows API)
  - Position cache (persists cursor position and open counts across sessions)
  - CLI flags: `--help`, `--edit`, `--health` (`checkhealth`)
  - Thread-safe event manager and buffer concurrency protection

- **Macros & Repeat**
  - Macro recording and playback (`q[a-z]`, `@[a-z]` with count support, e.g. `3@a`)
  - Dot-repeat (`.`) via `LastRepeatableFn` with count preservation (`2dd` → `.` deletes 2)
  - Count support on `dd`, `dw`, `x`, `yy`
  - Repeatable Ex commands (`:cn`, `:cp`, git hunks) via `Repeatable` flag
  - Recursion guard preventing `.` inside macro playback

- **Configuration**
  - `~/.config/wig/config.toml`:
    - `[editor]`: theme, line numbers, format-on-save, git view modes, indent guides, LSP toggle, which-key format
    - `[keys]`: custom mode keymaps with command resolution from `wig.AllCommands`
    - Ability to disable / unbind default keybindings via `CmdDummyNA`
  - `~/.config/wig/languages.toml`:
    - LSP definitions, command arguments, and language-specific indentation settings
  - Runtime theme switcher (`<Leader>t`) and virtual indent guides toggle (`<Leader>i`)

- **Visual Block**
  - Rectangle selection mode (`Ctrl-v`)
  - Block insert (`I`), block yank (`y`), block delete (`d`/`x`)

- **Result Navigation**
  - `:cn` and `:cp` to navigate search/diagnostic results (dot-repeatable via `Repeatable` flag)

- **Command Line & Ex Mode**
  - Line jump (`:123`), file open (`:e <file>`), toast notification (`:info [msg]`)
  - Command argument parsing
  - Search & replace (`:s/...`, `:%s/...`, `:'<,'>s/...` with `c` and `g` flags)
  - Full line editing shortcuts: `Ctrl-a`, `Ctrl-e`, `Ctrl-w`, `Ctrl-r Ctrl-w`

- **Completion**
  - LSP completions
  - Local buffer word completion fallback (`Alt-/`)
  - Single-candidate automatic completion

---

## 🚧 TODO: To be a "Real Vim"

### High Priority (Core Vim Features)
- [v] **Ex Mode / Command-line ranges**: `:[range]s/old/new/g`, `:%s/...`, `:'<,'>s/...` with `c` and `g` flags
- [v] **Search & Replace**: Visual selection + `:s/.../...`, `:%s/.../...` with confirmation (`c` flag)
- [v] **Dot Repeat & Count**: `.` repeats last change with count preservation (`2dd`→`.` deletes 2)
- [ ] **Horizontal Splits**: `:sp` / `ctrl+w s` (currently only vertical splits supported)
- [v] **Marks**: `m[a-z]` to set mark, `` `[a-z] `` to jump to mark, ``` `` ``` for last position
- [v] **Advanced Text Objects**: `ci(`/`ci{`/`ci[`/`ci'`/`ci"`, `ca(`/`ca{`, `di(`/`da(`, `yi(`/`ya(`, `ciw`/`caw`
- [v] **Quickfix List**: `:copen` opens LSP diagnostics in a split buffer (navigate with `:cn`/`:cp` or Enter)

### Medium Priority (Quality of Life)
- [ ] **Sessions**: Save & restore workspace sessions (`:mksession`, layout, open buffers, cursor locations)
- [ ] **Folding**: `zc`, `zo`, `za` with syntax/indent based folding
- [ ] **Tabs**: Vim-style tabs (`:tabnew`, `gt`, `gT`)
- [ ] **Jumplist**: `Ctrl-o` (jump back), `Ctrl-i` (jump forward) across all jumps
- [ ] **Mouse Support**: Click to set cursor, drag to select visual mode, scroll wheel
- [v] **System Clipboard Integration**: Copy/paste via `<Leader>y`, `<Leader>pv`, `<Leader>pp` and bracketed paste
- [v] **Named & System Registers**
  - Register popup (`"` / `:reg` / `:registers`)
  - Preview
  - `%` current file
  - `+`/`*` system clipboard
  - `0` last yank
  - `1`–`9` history
- [ ] **Word Motions**: `e` (end of word), `ge` (end of word backward), `W`, `B`, `E` (WORD motions)
- [ ] **Multi-cursor**: `Ctrl-n`/`Ctrl-v` multi-edit

### Low Priority (Advanced / Plugins)
- [ ] **Terminal Emulator**: Built-in terminal (`:term`)
- [ ] **Tags**: Ctags integration (`:tag`, `Ctrl-]`, `Ctrl-t`)
- [ ] **Diff Mode**: `vimdiff` support
- [ ] **Statusline Customization**: `set statusline=...` or config equivalent
- [ ] **Inline LSP Actions**: Rename (`<leader>rn`), Code Actions (`<leader>ca`)
- [ ] **Snippets UI**: UI for selecting between multiple snippet matches

---

## 💡 Future Plans / Ideas
- **Better TUI**: Inline widgets for renames, floating window improvements
- **Performance**: Optimize rendering to only redraw changed cells instead of full screen `Fill`
