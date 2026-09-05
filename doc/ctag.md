# Tag Navigation and Definition Specification

## Overview
This document specifies the behavior for tag navigation and "Go to Definition" functionality in the editor. The system uses a tiered fallback architecture to balance precision, performance, and availability.

## Fallback Priority
When the user triggers "Go to Definition" (`gd` / `CmdGotoDefinition`) or a tag jump (`CmdTagJump`, or `CmdTag` with cursor-derived context), the editor resolves the target location using the following strict priority:

1. **LSP (Language Server Protocol)**: Provides precise, context-aware definitions based on semantic analysis.
2. **ctagd Daemon (context-aware)**: A fast, background indexing service reached over a Unix socket, queried with file/line/column/symbol context so it can disambiguate identically-named symbols. Used when LSP is unavailable or returns no result.
3. **Local Tags File**: A legacy fallback parsing a statically generated `tags` file using `ctags`.

Both `CmdGotoDefinition` and `CmdTagJump` implement this same three-tier chain independently — either is a valid entry point and both end up trying LSP, then ctagd, then the local tags file, in that order.

## 1. LSP (Primary)
* **Entry Points**:
  * `CmdGotoDefinition(ctx wig.Context)` in `commands/commands.go`.
  * `lspJumpToDefinition(ctx wig.Context) bool` in `commands/tags.go`, used internally by `CmdTagJump` and by `CmdTag`'s cursor-derived path.
* **Mechanism**: Queries the active language server via `ctx.Editor.Lsp.Definition(ctx.Buf, *cur)`.
* **Behavior**:
  * If LSP returns a valid file path and cursor position, the editor opens the file and moves the cursor there.
  * If LSP is not running, not started, or returns an empty result:
    * `CmdGotoDefinition` falls back by calling `CmdTagJump(ctx)`, which independently re-runs the full lsp → ctagd → tags chain (LSP is tried again but will again yield nothing, then it proceeds to Tier 2).
    * `lspJumpToDefinition` simply returns `false`, letting its caller (`CmdTagJump` / `CmdTag`) proceed directly to Tier 2 without a redundant LSP retry.
  * Echoes/logs status messages indicating the LSP query and whether a fallback is occurring.

## 2. ctagd Daemon (Secondary)
Two lookup modes exist, selected by whether cursor context is available:

* **Context-aware lookup (preferred whenever a cursor position is available)**
  * **Entry Point**: `ctagdDefinitionAndJump(ctx wig.Context, echo bool) bool` in `commands/ctagd.go`, backing `CmdCtagdGotoDefinition` and used by `CmdTagJump` / `CmdTag`'s cursor-derived path.
  * **Mechanism**: Sends a `definition` request including repo root, the file's path relative to the repo root, the cursor's line/column, and the symbol name under the cursor.
  * **Rationale**: Because the request carries file+line+column, the daemon can resolve the specific occurrence under the cursor rather than an arbitrary symbol sharing the same name elsewhere in the codebase.
  * Returns `false` on any failure (no repo root, connection error, null result, or malformed response) so callers can fall through to Tier 3; `echo` controls whether failures are reported to the user via `EchoMessage` or fail silently.
* **Name-only lookup (fallback when no cursor context exists)**
  * **Entry Point**: `ctagdGotoAndJump(ctx wig.Context, word string) bool` in `commands/ctagd.go`, used by `CmdTag` only when a `<word>` argument is typed explicitly (e.g. `:tag SomeFunc`) with no cursor-derived context to send.
  * **Mechanism**: Sends a `goto` request with only the repo root and the query string (symbol name).
  * **Caveat**: Because there is no positional context, if the daemon's index contains multiple symbols with the same name, the daemon's own ranking decides which one is returned — the editor cannot disambiguate. Prefer the context-aware path whenever a cursor position is available.
* **Location adjustment**: `ctagdJumpToLocation` treats `line`/`column` in the daemon's response as already 0-based (matching `wig.Cursor` and the LSP convention ctagd is built on) and does **not** subtract 1. This differs from the local tags file below, which is 1-based.
* **Background Updates**: 
  * On file save (`OnSaveHook` -> `CmdCtagdSaved`), the buffer content is asynchronously sent to `ctagd` via a `saved` request to keep the daemon's index up-to-date without blocking the UI.

## 3. Local Tags File (Tertiary)
* **Entry Point**: `jumpToTagName(ctx, word)` in `commands/tags.go`
* **Mechanism**:
  * Looks for a `tags` file in the project root (`findTagRoot`).
  * If missing, attempts to generate it synchronously using `exec.Command("ctags", "-R", ...)` (legacy behavior).
  * Parses the file using `loadTags(rootDir)`, utilizing an mtime-based cache (`tagsCache`).
* **Behavior**:
  * If exact matches are found, it jumps to the location.
  * If multiple matches are found, it opens a Picker UI (`ui.PickerInit`) allowing the user to select the desired tag.
  * Location resolution prioritizes the line number, treating it as 1-based (converted to 0-based for `wig.Cursor` via a `-1` adjustment in `jumpToTag`). If the line number is invalid (`<= 0`), it falls back to pattern matching (`findLineByPattern`) using the `addr` field from the tags file.

## Additional Commands

### `:tag <word>` / word-under-cursor (`CmdTag`)
* **Behavior**:
  * If a `<word>` argument is provided explicitly (no cursor context available), it attempts `ctagdGotoAndJump` (Tier 2, name-only), then `jumpToTagName` (Tier 3).
  * If no `<word>` is provided, it extracts the word under the cursor. When a word is found, it runs the full context-aware chain: `lspJumpToDefinition` (Tier 1) → `ctagdDefinitionAndJump` (Tier 2, context-aware, silent) → `jumpToTagName` (Tier 3).
  * If no word is under the cursor, it forces a synchronous update of the local `tags` file via `updateTagsFile` and clears the cache.

### `gd` under cursor (`CmdTagJump`)
* **Behavior**: Extracts the word under the cursor and runs the full chain: `lspJumpToDefinition` (Tier 1) → `ctagdDefinitionAndJump` (Tier 2, context-aware, silent) → `jumpToTagName` (Tier 3). This is the command `CmdGotoDefinition` falls back to when LSP alone yields nothing.

### Workspace Symbols (`CmdCtagdWorkspaceSymbols`)
* Queries the `ctagd` daemon for all symbols matching a query string.
* Displays results in a Picker UI.
* Does *not* fall back to local tags, as local tags files are not typically structured for fast fuzzy workspace symbol searching.

## Configuration & Dependencies
* **LSP**: Requires `lsp_enabled = true` in `config.toml` and a valid language server configuration for the file type.
* **ctagd**: Requires the `ctagd` binary to be in the system `PATH`. The daemon is automatically started by the editor if a connection is attempted and the binary is found.
* **Local Tags**: Requires the `ctags` binary (Universal Ctags or Exuberant Ctags) to be in the system `PATH` for file generation.
