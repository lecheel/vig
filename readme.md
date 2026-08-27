# Vig (written in Go)

**Vig** is a modal, Vim‑like text editor written in Go, forked from the excellent [Wig](https://github.com/wig-editor/wig) project.  
It builds upon Wig’s solid foundation with a **focus on stability, core Vim operations, and a clean daily‑driver experience**.

<p align="center">
  <img src="output.gif" alt="Vig preview" />
</p>

---

## ✨ Features

Vig implements a wide range of Vim‑style editing capabilities, many inherited from Wig and further refined:

- **Modes** – Normal, Insert, Visual, Visual Line, Visual Block.
- **Core Editing** – Text insertion/deletion, Myers diff‑based undo/redo, yank/put with rectangle block support, auto‑indentation, comment toggling (`gcc` etc.), delete to end of file (`dG`), boolean toggle (`toggle`), and extended comment motions (`gcj`, `gck`, `gc$`, `gcG`, `gcgg`, `gcip`, `gcap`).
- **Movement & Navigation** – Standard Vim motions (`h/j/k/l`, `w/b/e`, `0/$`, `%`, `f/F/t/T`), scrolling, EasyMotion (`s` with home‑row overlays), end‑of‑file jump (`G`), and mark support (`m[a-z]`, `` ` `` popup with line preview).
- **Search & Replace** – Incremental search (`/`) with live highlighting and cursor preview, `n`/`N` navigation, project‑wide search via ripgrep (`<Leader>f`, `:s`, `:rg`), word‑under‑cursor search, and **real‑time highlighting for substitution commands** (`:s/…`) with support for escaped delimiters.
- **File & Buffer Management** – Open/save files (auto‑creation), buffer picker, MRU buffer picker, file picker, command palette, `config` command to open `~/.config/vig/config.toml`, and CLI flags (`--gf`, `--gs`) to open pickers on startup.
- **Window Management** – Vertical/horizontal splits (`:vs`, `:sp`), window cycling (`<C-w>w`), closing (`:close`, `:only`), and layout toggling.
- **Git Integration** – Status panel (`gs`) with staging (`s`), diff inspection (`d`), refresh (`r`); interactive commit editor; remote push (`p`); stash management (`z`) with pop/drop/cancel; diff highlighter; gutter signs (`+`, `-`, `~`) and hunk navigation (`]h`/`[h`), preview, and revert; inline Git blame; **AI commit message generation** (`git-ai`); and a `gfiles` picker to toggle between status and last commit views.
- **LSP (Language Server Protocol)** – Autocompletion (auto‑trigger), go to definition (`gd`, `gd` in other window), hover docs, signature help, diagnostics/quickfix (`:copen`), LSP status (`:LspStatus`), and decoupled quickfix population.
- **Syntax Highlighting** – Tree‑sitter support for **Go, Rust, Odin, C, Python, Bash, JSON, TOML** (with shebang detection) and custom query support via `~/.config/vig/queries/`.
- **UI & Interaction** – Statusline with mode, file status, cursor coords; notification toasts; Which‑Key popup; EasyMotion overlay; command line with prompt and history; hover/autocomplete popups; quickfix popup widget; marks popup; and **autocomplete navigation with arrow keys**.
- **Command Line & Ex Mode** – Full command‑line editing (Home/End, `Ctrl‑a/e/u/k/w`), `Ctrl‑r Ctrl‑w` to insert word under cursor, visual‑mode prefilled ranges (`'<,'>`), register insertion (`Ctrl‑r`), and line jump (`:123`).
- **Registers & Macros** – Named registers (`a-z`), unnamed (`"`), yank history (`0-9`), system clipboard (`+`, `*`), current file (`%`), search pattern (`/`), last inserted text (`.`), last command (`:`), and macro recording/playback (`q[a-z]`, `@[a-z]`) with count support.
- **Repeat & Count** – Dot‑repeat (`.`) with count preservation (`2dd` → `.` deletes 2), repeatable Ex commands (`:cn`, `:cp`, git hunks).
- **Configuration** – `~/.config/vig/config.toml` for theme, line numbers, format‑on‑save, indent guides, LSP toggle, leader key, multi‑key sequences, and more; `~/.config/vig/languages.toml` for LSP definitions and indentation settings. Runtime theme switcher (`<Leader>t`) and indent guides toggle (`<Leader>i`).
- **Visual Block** – Rectangle selection (`<C-v>`), block insert (`I`), block yank (`y`), block delete (`d`/`x`).
- **System Integration** – System clipboard (`<Leader>y`, `<Leader>pv`, `<Leader>pp`), bracketed paste, cross‑platform clipboard health check, position cache, thread‑safe event manager, and CLI support (`--help`, `--edit`, `--health`).

---

## 🚀 Getting Started

### Build & Run

```bash
make setup-runtime
make build-run
```

Then edit your configuration file (it will be created automatically if missing):

```bash
vim ~/.config/vig/config.toml   # or use vig's built‑in `:config` command
```

### Dependencies

- Go 1.21+
- Tree‑sitter parsers (fetched via `make setup-runtime`)
- ripgrep (for project search)
- Git (for git features)

---

## ⌨️ Keybindings

Most common Vim keybindings are implemented. Here are some highlights:

| Key                | Description                         |
|--------------------|-------------------------------------|
| `Tab` / `Shift-Tab`| Next / previous item in popup       |
| `Space + f`        | Find files in Git project           |
| `Space + b`        | Show buffers                        |
| `Space + s + s`    | Fuzzy text search                   |
| `Space + `` ` ``   | Toggle between two files            |
| `Space + /`        | Search text in project (ripgrep)    |
| `gcc`              | Toggle comment on current line      |
| `gc` + motion      | Toggle comment on motion            |
| `]h` / `[h`        | Next / previous git hunk            |
| `<Leader>t`        | Change theme                        |
| `<Leader>i`        | Toggle indent guides                |
| `gd`               | Go to definition                    |

For a complete list, see `config/config.go` or use the built‑in **Which‑Key** helper (press `<Leader>` to trigger).

---

## 🙏 Acknowledgements

Vig would not exist without the incredible work of the **Wig** community.  
We are deeply grateful to all [Wig contributors](https://github.com/wig-editor/wig/graphs/contributors) for creating such a solid and inspiring editor.

<a href="https://github.com/wig-editor/wig/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=wig-editor/wig" />
</a>

Vig aims to carry forward Wig’s vision while focusing on **stability, reliability, and a familiar Vim experience** for daily use.  
Special thanks to everyone who has contributed to Vig directly – your feedback and patches are invaluable.

---

## 📖 More Information

- **Plans** – Vig will continue to polish core features, improve performance, and add user‑requested enhancements while keeping the editor lightweight and fast.
- **Issues / Contributions** – Please report bugs and send pull requests on our GitHub repository. We welcome contributions of all kinds!

Happy editing! 🖊️
