# Wig - TODO & Roadmap

## ✅ Currently Implemented
- **Modes**: Normal, Insert, Visual, Visual Line, Visual Block.
- **Core Editing**: Text insert/delete, Undo/Redo (Myers diff based), Yank/Put (with block support), Auto-indent, Comment toggle.
- **Movement**: `h/j/k/l`, `w/b`, `%` (match pair), `f/F/t/T` (find to), `0/$`, Page Up/Down, Scroll Up/Down.
- **Marks**: `m[a-z]` to set, `` ` `` popup with mark legends & line hints, `` `[a-z] `` to jump, ``` `` ``` pingpong jump toggle. Visual marks displayed in the gutter (yellow).
- **Search**: Basic search (`/`), next/prev (`n`/`N`), project-wide ripgrep search, word under cursor search.
- **File Management**: Open file, Save, Buffer picker, MRU Buffer Picker, File picker, Command Palette. Creating new files from command line if they don't exist.
- **Window Management**: Vertical split, Window next (`ctrl+w w`), Close window, Close other, Close & kill buffer, Toggle layout.
- **Git Integration**: Git gutter signs (ignores untracked files), Hunk navigation (`]h`/`[h`), Hunk revert/preview, Inline Git Blame, Blame commit detail, Git status panel. `l`/`L` jumps to the first item of the next/previous session in git status.
- **LSP**: Completion, Goto definition, Hover, Signature help, Diagnostics.
- **Syntax Highlighting**: Tree-sitter (Go, Rust, Odin, C, Python).
- **UI Elements**: Statusline, Which-Key popup, Command line, Search prompt, Hover popup, Autocomplete, Mini Help, Confirm prompts, Marks popup.
- **System**: System clipboard support (yank/paste, bracketed paste), Position cache (remember cursor on reopen), Basic macros, CLI flags (`--help`, `--edit`, `--health`).
- **Configuration**: `config.toml` (settings, keys), `languages.toml` (LSP/Indent), Themes. Ability to disable keys via `CmdDummyNA`.
- **Visual Block**: `Ctrl-v` rectangle selection, block insert (`I`), block yank (`y`), block delete (`d`/`x`).
- **Result Navigation**: `:cn` and `:cp` to jump through search results.
- **Command Line**: `:123` to jump to line, `:e <file>` to open/create files, basic Ex commands.
- **Completion**: LSP completions, manual local buffer completions (`Alt-/`), single-candidate auto-complete.

---

## 🚧 TODO: To be a "Real Vim"

### High Priority (Core Vim Features)
- [ ] **Ex Mode / Command-line ranges**: Implement `:[range]s/old/new/g`, `:%s/...`, `:1,10d`, etc. Currently `:` only executes simple commands.
- [ ] **Search & Replace**: Visual selection + `:s/.../...`, `:%s/.../...` with confirmation (`c` flag).
- [ ] **Horizontal Splits**: `:sp` / `ctrl+w s`. Currently only vertical splits are supported.
- [v] **Marks**: m[a-z] to set mark, `[a-z] to jump to mark, `` for last position
- [ ] **Advanced Text Objects**: `ci(`, `ci{`, `ci'`, `da"`, `ciw` inside punctuation/blocks.
- [ ] **Quickfix List**: `:copen`, `:cnext`, `:cprev` for LSP diagnostics and grep results.

### Medium Priority (Quality of Life)
- [ ] **Folding**: `zc`, `zo`, `za` with syntax/indent based folding.
- [ ] **Tabs**: Vim-style tabs (`:tabnew`, `gt`, `gT`).
- [ ] **Jumplist**: `Ctrl-o` (jump back), `Ctrl-i` (jump forward) across all jumps, not just explicit `Jumps` list.
- [ ] **Mouse Support**: Click to set cursor, drag to select visual mode, scroll wheel.
- [ ] **Registers**: Multiple registers (`"ay`, `"ap`), system clipboard registers (`"+`, `"*`).
- [ ] **Word Motions**: `e` (end of word), `ge` (end of word backward), `W`, `B`, `E` (WORD motions).
- [ ] **Multi-cursor**: Visual block insert is there, but `Ctrl-n`/`Ctrl-v` multi-edit would be great.

### Low Priority (Advanced / Plugins)
- [ ] **Terminal Emulator**: Built-in terminal (`:term`).
- [ ] **Tags**: Ctags integration (`:tag`, `Ctrl-]`, `Ctrl-t`).
- [ ] **Diff Mode**: `vimdiff` support.
- [ ] **Statusline Customization**: `set statusline=...` or Lua/eval equivalent.
- [ ] **Inline LSP Actions**: Rename (`<leader>rn`), Code Actions (`<leader>ca`).
- [ ] **Snippets UI**: UI for selecting between multiple snippet matches.

---

## 💡 Future Plans / Ideas
- **Better TUI**: Inline widgets for renames, floating window improvements.
- **Performance**: Optimize rendering to only redraw changed cells instead of full screen `Fill`.
