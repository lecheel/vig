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
  - **Delete to end of file (`dG`, `CmdDeleteEndOfFile`)**
  - **Boolean toggle command (`toggle`, toggles true/false)**
  - **Extended comment motions**: `gcc`, `gcj`, `gck`, `gc$`, `gcG`, `gcgg`, `gcip`, `gcap` with paragraph and text‑object support
  - **Save with feedback** (lines/bytes written, visual toast) and **save-as** (`:w <filename>`)
  - **Line and selection indentation** (`>`, `<` with LSP-based indent units)

- **Movement**
  - Directional navigation: `h`, `j`, `k`, `l`, `w`, `b`, `0`, `$`
  - Target jumps: `%` (matching pair), `f`/`F`/`t`/`T` (find before/to character)
  - Scrolling: Page Up/Down, Line Up/Down
  - EasyMotion: `s` 2-character viewport jump with home-row target overlays (Normal and Visual)
  - **End‑of‑file jump (`G`)**
  - **Function navigation**: `gn`/`gN` to jump to next/previous function definition (dot-repeatable)

- **Marks**
  - `m[a-z]` to set local buffer marks
  - `` ` `` popup with mark legends and line preview hints
  - `` `[a-z] `` to jump to mark, ``` `` ``` for ping-pong jump toggle
  - Visual mark indicators displayed in the line number gutter (yellow)

- **Search**
  - Incremental search (`/`) with live match highlighting and cursor preview
  - Result navigation with `n` (next) and `N` (previous) – **selection is extended during searches**
  - Project-wide search via ripgrep (`<Leader>f`, `:s`, `:rg`)
  - Word under cursor search (`CmdSearchWordUnderCursor`, `CmdRgUnderCursor`)
  - **Real‑time highlighting for substitution commands** (`:s/…`) while typing
  - **Support for escaped delimiters** (`\/`) in search & replace patterns

- **File Management**
  - Open and save files (with auto-creation when path does not exist)
  - Buffer picker (`CmdBufferPicker`)
  - Most Recently Used buffer picker (`CmdMRUBufferPicker`)
  - File picker and command palette
  - **`config` command to open `~/.config/wig/config.toml` (creates if missing)**
  - **`--gf` (git files) and `--gs` (git status) CLI flags to open pickers on startup**

- **Window Management & Workspaces**
  - **Recursive window splits**: Nested horizontal/vertical layouts (`WinNode` tree)
  - Vertical split (`:vs`, `CmdWindowVSplit`) and Horizontal split (`:sp`, `CmdWindowHSplit`) with max limits
  - Window cycling (`ctrl+w w`, `CmdWindowNext`)
  - Window closing (`:close`, `:only`, close & kill buffer)
  - Layout toggling between horizontal and vertical layouts
  - **Workspace sessions**: Up to 10 workspaces (`wslist`), move windows between them
  - **Workspace persistence**: Save/restore layouts and file history across sessions (`SaveWorkspaces` config)
  - **Persistence status indicator** (💾/🔒) in the statusline

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
  - **AI commit message generation** (`git-ai` integration, with spinner and cancellation via Esc/q)
  - **Git files picker** (`gfiles` command, toggles between status and last commit views)

- **LSP (Language Server Protocol)**
  - Autocompletion popup with auto-trigger
  - Go to definition (`gd`, `CmdGotoDefinition`, `CmdGotoDefinitionOtherWindow`)
  - Hover documentation popup (`K`, `CmdLspHover`)
  - Signature help (`CmdLspShowSignature`)
  - Document diagnostics and Quickfix list (`:copen`)
  - **LSP status command (`:LspStatus`, `:diagnostics` / `:diags` to populate quickfix)**
  - **Decoupled quickfix population from diagnostic viewing**

- **Syntax Highlighting**
  - Tree-sitter integration for **Go**, **Rust**, **Odin**, **C**, **Python**, **Bash**, **JSON**, **TOML**
  - **Shebang detection** for Python (and bash‑like shells)
  - Custom query support via `~/.config/wig/queries/<lang>/highlights.scm`

- **UI Elements**
  - Statusline with mode, file status, cursor coordinates, **current function name**, and **workspace persistence indicator** (💾/🔒)
  - Notification toasts (`ui.Notify` with 5s auto-dismiss, stacking, and type badges)
  - Which-Key popup (multi-column, mode title, configurable `words`/`camel`/`cmd` formats)
  - EasyMotion visual jump labels overlay
  - Command line, search prompt, hover popup, and autocomplete popup
  - Confirm prompts (`ui.ConfirmInit`) and marks popup (`ui.mark`)
  - Unified popup border and title styling (`ui.popup.title`)
  - **Quickfix popup widget** (alternative to split, with j/k navigation and direct jumps)
  - **Autocomplete navigation with arrow keys** (multi‑column grid, 2D candidate cycling)
  - **Command‑line cursor editing** (Home, End, Left, Right, Emacs‑style `Ctrl‑a/e/u/k/w`)
  - **`Ctrl‑r Ctrl‑w`** inserts the word under cursor into the command line

- **System & Integration**
  - System clipboard: copy (`<Leader>y`), paste at cursor (`<Leader>pv`), replace buffer (`<Leader>pp`)
  - Bracketed paste mode support (`tcell.EventPaste`)
  - Cross-platform clipboard health checks (`checkhealth`: `pbcopy`, `wl-clipboard`, `xclip`, `xsel`, Windows API)
  - Position cache (persists cursor position and open counts across sessions)
  - CLI flags: `--help`, `--edit`, `--health` (`checkhealth`)
  - Thread-safe event manager and buffer concurrency protection
  - **Comprehensive register system** with popup (`:reg` / `:registers`):
    - Named registers (`a-z`), unnamed (`"`), yank history (`0-9`)
    - System clipboard (`+`, `*`), current file (`%`), search pattern (`/`)
    - Last inserted text (`.`), last command (`:`), and `Ctrl‑r` insertion in insert/command modes

- **Macros & Repeat**
  - Macro recording and playback (`q[a-z]`, `@[a-z]` with count support, e.g. `3@a`)
  - Dot-repeat (`.`) via `LastRepeatableFn` with count preservation (`2dd` → `.` deletes 2)
  - Count support on `dd`, `dw`, `x`, `yy`
  - Repeatable Ex commands (`:cn`, `:cp`, git hunks) via `Repeatable` flag
  - Recursion guard preventing `.` inside macro playback

- **Configuration**
  - `~/.config/wig/config.toml`:
    - `[editor]`: theme, line numbers, format-on-save, git view modes, indent guides, LSP toggle, which-key format, **`notify_on_save`**, **`same_statusline_color`**
    - `[keys]`: custom mode keymaps with command resolution from `wig.AllCommands`
    - Ability to disable / unbind default keybindings via `CmdDummyNA`
    - **Configurable leader key** (`Leader` setting, supports `<leader>` placeholder)
    - **Multi‑key sequence support** (e.g., `<leader>f`, `<c-w>h`) with token‑based expansion
  - `~/.config/wig/languages.toml`:
    - LSP definitions, command arguments, and language-specific indentation settings
  - Runtime theme switcher (`<Leader>t`) and virtual indent guides toggle (`<Leader>i`)

- **Visual Block**
  - Rectangle selection mode (`Ctrl-v`)
  - Block insert (`I`), block yank (`y`), block delete (`d`/`x`)

- **Result Navigation**
  - `:cn` and `:cp` to navigate search/diagnostic results (dot-repeatable via `Repeatable` flag)
  - **Visit handlers for `rgcollect` and `quickfix` buffers**

- **Command Line & Ex Mode**
  - Line jump (`:123`), file open (`:e <file>`), toast notification (`:info [msg]`)
  - Command argument parsing
  - Search & replace (`:s/...`, `:%s/...`, `:'<,'>s/...` with `c` and `g` flags)
  - Full line editing shortcuts: `Ctrl-a`, `Ctrl-e`, `Ctrl-w`, `Ctrl-r Ctrl-w`
  - **Visual‑mode prefilled range** (`'<,'>`) when `:` is pressed from visual mode
  - **`Ctrl‑r` (register insertion) in command line**


- **Completion**
  - LSP completions
  - Local wordlist and buffer word completion fallback (`Alt-/` / `LocalComplete`)
  - **Path completion** (`./`, `../` filesystem traversal)
  - Single-candidate automatic completion

---

## 🚧 TODO: To be a "Real Vim"

### High Priority (Core Vim Features)
- [v] **Ex Mode / Command-line ranges**: `:[range]s/old/new/g`, `:%s/...`, `:'<,'>s/...` with `c` and `g` flags
- [v] **Search & Replace**: Visual selection + `:s/.../...`, `:%s/.../...` with confirmation (`c` flag)
- [v] **Dot Repeat & Count**: `.` repeats last change with count preservation (`2dd`→`.` deletes 2)
- [v] **Horizontal Splits**: `:sp` / `ctrl+w s`
- [v] **Marks**: `m[a-z]` to set mark, `` `[a-z] `` to jump to mark, ``` `` ``` for last position
- [v] **Advanced Text Objects**: `ci(`/`ci{`/`ci[`/`ci'`/`ci"`, `ca(`/`ca{`, `di(`/`da(`, `yi(`/`ya(`, `ciw`/`caw`
- [v] **Quickfix List**: `:copen` opens LSP diagnostics in a split buffer (navigate with `:cn`/`:cp` or Enter)

### Medium Priority (Quality of Life)
- [ ] **Sessions**: Save & restore workspace sessions (`:mksession`, layout, open buffers, cursor locations)
- [ ] **Folding**: `zc`, `zo`, `za` with syntax/indent based folding
- [ ] **Tabs**: Vim-style tabs (`:tabnew`, `gt`, `gT`)
- [ ] **Jumplist**: `Ctrl-o` (jump back), `Ctrl-i` (jump forward) across all jumps *(basic search‑based jump list exists)*
- [ ] **Mouse Support**: Click to set cursor, drag to select visual mode, scroll wheel
- [v] **System Clipboard Integration**: Copy/paste via `<Leader>y`, `<Leader>pv`, `<Leader>pp` and bracketed paste
- [v] **Named & System Registers** (complete with popup, yank history, system clipboard, etc.)
- [ ] **Word Motions**: `e` (end of word), `ge` (end of word backward), `W`, `B`, `E` (WORD motions)
- [ ] **Multi-cursor**: `Ctrl-n`/`Ctrl-v` multi-edit

### Low Priority (Advanced / Plugins)
- [ ] **Terminal Emulator**: Built-in terminal (`:term`)
- [v] **Tags**: Ctags integration (`:tag`, `Ctrl-]`, `Ctrl-t`) with caching and picker for ambiguous matches
- [ ] **Diff Mode**: `vimdiff` support
- [ ] **Statusline Customization**: `set statusline=...` or config equivalent
- [ ] **Inline LSP Actions**: Rename (`<leader>rn`), Code Actions (`<leader>ca`)
- [ ] **Snippets UI**: UI for selecting between multiple snippet matches

---

## 💡 Future Plans / Ideas
- **Better TUI**: Inline widgets for renames, floating window improvements
- **Performance**: Optimize rendering to only redraw changed cells instead of full screen `Fill`
