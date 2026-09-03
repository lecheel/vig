### 1.1 Data Structures

Multi-cursor state is tracked per `Buffer` via the `MultiCursor` struct:

```go
type CursorInstance struct {
    Cursor    Cursor
    Selection *Selection
}

type MultiCursor struct {
    buf          *Buffer
    Cursors      []CursorInstance
    Pattern      string
    PrimaryIndex int
}

  - Cursors: Slice of CursorInstance. Each instance holds its own Cursor (line,
    character column, and preserved horizontal column) and an optional Selection
    (start and end bounds).
  - Pattern: The substring or word currently being matched. Set on the initial
    invocation of CmdMultiCursorMatchNext.
  - PrimaryIndex: Zero-based index pointing to the primary selection/cursor that
    anchors viewport focus and window cursor synchronization.
  - Active(): Returns true if len(Cursors) > 1 || (len(Cursors) == 1 && Pattern
    != "").
  - Primary Cursor Synchronization: In all modes, the window's primary cursor
    (*ContextCursorGet(ctx)) strictly mirrors m.Cursors[m.PrimaryIndex].Cursor.
    The viewport is anchored to this cursor and never jumps to off-screen
    secondary cursors during typing or collapsing.

2. Modes and State Transitions

2.1 State Diagram

                 [ Normal Mode (single cursor) ]
                                |
                   ctrl+d (on word or selection)
                                v
               [ Multi-Cursor Visual Mode (MODE_VISUAL) ]
                 - Multiple selections highlighted
                 - Pattern registered in MultiCursor
                 - Viewport anchored to PrimaryIndex
                 /       |              |          \
         ctrl+d / ctrl+k |     ) or (   |           \ c / i / a / d
               v         v              v            v
          [ Add ]    [ Skip ]      [ Rotate ]   [ Edit / Insert Mode ]
                                  (cycles view)       |
                                                  Esc |
                                                      v
                                        [ Normal Mode (single cursor) ]

2.2 Mode Behaviors

| Mode          | Trigger / Transition                 | Multi-Cursor State                                                                 |
| ------------- | ------------------------------------ | ---------------------------------------------------------------------------------- |
| `MODE_NORMAL` | Pressing `Esc` from visual/insert    | Clears all secondary cursors via `m.Clear()`                                       |
| `MODE_VISUAL` | `CmdMultiCursorMatchNext` (`ctrl+d`) | All matched ranges have an active `Selection` in `m.Cursors`                       |
| `MODE_INSERT` | `c`, `i`, or `a` from visual mode    | Selections collapse to insertion points; typed input replicates across all cursors |

3. Command Specifications

3.1 CmdMultiCursorMatchNext (ctrl+d)

  - Initial Call (no active multi-cursor):
      - If a visual selection already exists (ctx.Buf.Selection != nil), the
        selected text becomes m.Pattern.
      - If no selection exists, extracts the word under the cursor via
        TextObjectWord(ctx, true) and sets m.Pattern.
      - Creates the first CursorInstance at the current word/selection with
        PrimaryIndex = 0.
      - Automatically searches forward (wrapping around EOF) and appends the
        next matching occurrence as the second CursorInstance.
      - Sets m.PrimaryIndex = len(m.Cursors) - 1, sets editor mode to
        MODE_VISUAL, centers the viewport on the new match, and echoes "<N>
        selections".
  - Subsequent Calls:
      - Searches forward starting from the end of the last selection.
      - Skips already selected occurrences.
      - If a new match is found, appends it to m.Cursors, updates m.PrimaryIndex
        = len(m.Cursors) - 1, updates ctx.Buf.Selection and the window cursor,
        and centers the view.
      - If no more unique matches exist, echoes "no more matches".

3.2 CmdMultiCursorSkipNext (ctrl+k)

  - Removes the most recently added cursor from m.Cursors.
  - Continues searching forward from the end of the skipped occurrence for the
    next available match.
  - If a subsequent match is found, adds it in place of the skipped one and sets
    m.PrimaryIndex = len(m.Cursors) - 1.
  - If no subsequent match is found, bounds-checks m.PrimaryIndex and gracefully
    restores ctx.Buf.Selection and the window cursor to
    m.Cursors[m.PrimaryIndex].

3.3 Selection Rotation: ) and ( (Helix-style Viewport Cycling)

Allows cycling primary focus through off-screen and on-screen selections without
dropping multi-cursor mode:

  - ) (CmdMultiCursorRotateForward):
      - Increments m.PrimaryIndex = (m.PrimaryIndex + 1) % len(m.Cursors).
      - Anchors the active window cursor and ctx.Buf.Selection to the new
        primary selection.
      - Calls CmdCursorCenter(ctx) to smoothly center the viewport on the
        selected match.
      - Displays "<PrimaryIndex>/<Total> selections" in the echo area.
  - ( (CmdMultiCursorRotateBackward):
      - Decrements m.PrimaryIndex = (m.PrimaryIndex - 1 + len(m.Cursors)) %
        len(m.Cursors).
      - Anchors the active window cursor and ctx.Buf.Selection to the new
        primary selection.
      - Calls CmdCursorCenter(ctx) to smoothly center the viewport on the
        selected match.
      - Displays "<PrimaryIndex>/<Total> selections" in the echo area.

3.4 Entering Insert Mode (i, a, c)

  - i (CmdEnterInsertMode):
      - Invokes m.CollapseToInsert(atEnd=false).
      - Moves each cursor to the start of its selection (sel.Start), clears each
        Selection, sets ContextCursorGet(ctx) to
        m.Cursors[m.PrimaryIndex].Cursor, and enters MODE_INSERT.
  - a (CmdEnterInsertModeAppend):
      - Invokes m.CollapseToInsert(atEnd=true).
      - If a selection is active, moves each cursor to sel.End.Char + 1.
      - If selections are already collapsed (Normal Mode), advances each cursor
        right by 1 character (clamped to line length).
      - Sets ContextCursorGet(ctx) to m.Cursors[m.PrimaryIndex].Cursor and
        enters MODE_INSERT.
  - c (CmdSelectionChange):
      - Calls m.DeleteSelections(ctx).
      - Deletes all selections across all cursors in a single undo transaction.
      - Sets ContextCursorGet(ctx) to m.Cursors[m.PrimaryIndex].Cursor (at the
        start of its deletion).
      - Enters MODE_INSERT.

3.5 Deletion (d, x)

  - Calls SelectionDelete(ctx), which delegates to m.DeleteSelections(ctx).
  - Deletions are processed in reverse document order (bottom-to-top,
    right-to-left) to prevent coordinate shifting from invalidating preceding
    ranges.
  - Subsequent cursor columns on the same line are shifted left by the deleted
    character count.
  - Synchronizes ContextCursorGet(ctx) to m.Cursors[m.PrimaryIndex].Cursor.
  - Clears selections and resets mode to MODE_NORMAL.

3.6 Escape (Esc)

  - Exiting to MODE_NORMAL via CmdNormalMode:
      - From MODE_INSERT: Adjusts all cursors left by 1 character (standard
        cursor positioning), anchors ContextCursorGet(ctx) to
        m.Cursors[m.PrimaryIndex].Cursor, and clears active selections while
        retaining multiple cursors.
      - From MODE_VISUAL or MODE_NORMAL: Calls m.Clear(), resetting the buffer
        back to a single standard cursor.

4. Input & Text Manipulation (insert.go)

When ctx.Buf.MultiCursor.Active() is true during MODE_INSERT, keypresses are
intercepted by m.HandleInsertKey(ctx, ev) before standard single-cursor logic
executes.

4.1 Supported Operations

1.  Rune Insertion:
      - Sorts cursors descending while preserving PrimaryIndex identity.
      - For each cursor, calls TextInsert(m.buf, line, pos, strToInsert).
      - Shifts subsequent cursors on the same line right by
        utf8.RuneCountInString(strToInsert).
      - Anchors window cursor to m.Cursors[m.PrimaryIndex].Cursor and keeps it
        visible via CmdEnsureCursorVisible.
2.  Backspace (KeyBackspace, KeyBackspace2):
      - Sorts cursors descending while preserving PrimaryIndex.
      - For each cursor at column > 0, deletes the preceding character using
        TextDelete.
      - Shifts subsequent cursors on the same line left by 1 character.
      - Anchors window cursor to m.Cursors[m.PrimaryIndex].Cursor.
3.  Enter / Newline (KeyEnter, KeyCtrlJ):
      - Handled explicitly before modifier checking so KeyCtrlJ (\n) is never
        dropped by ModCtrl guards.
      - Inserts \n at each cursor position.
      - Advances cursor to Line + 1, Char = 0.
      - Subsequent cursor line indices are adjusted accordingly.
4.  Tab (\t):
      - Inserts the configured indentation unit (\t or spaces) across all
        cursors.

4.2 Ignored Modifiers

Key combinations with Ctrl, Alt, or Meta (except KeyCtrlJ and KeyEnter) return
false, allowing normal command shortcuts and escape sequences to pass through.

5. UI and Rendering Specification (ui/window.go & ui/statusline.go)

5.1 Selection Rendering

Inside WindowRender, each character cell (lineNum, i) queries multi-cursor
state:

if buf.MultiCursor != nil && buf.MultiCursor.Active() {
    if buf.MultiCursor.HasSelectionAt(lineNum, i) {
        textStyle = wig.ApplyBg("ui.selection.primary", textStyle)
    }
} else if buf.Selection != nil {
    ...
}

  - Every active selection range across all cursor instances is rendered using
    the ui.selection.primary style background.
  - Preserves syntax highlighting foreground colors (textStyle) while applying
    the selection background.

5.2 Cursor Rendering

Each character cell checks for cursor presence:

isCursor := false
if buf.MultiCursor != nil && buf.MultiCursor.Active() {
    isCursor = buf.MultiCursor.HasCursorAt(lineNum, i)
} else {
    isCursor = (lineNum == cur.Line && i == cur.Char)
}

  - Renders cursor glyphs with mode-specific styles (ui.cursor.primary.insert,
    ui.cursor.primary.select, or ui.cursor).
  - Handles cursors positioned past the last visible character of a line
    (cur.Char >= len(currentLine.Value)) by emitting a trailing space styled
    with ui.cursor.

5.3 Statusline Integration (ui/statusline.go)

When buf.MultiCursor.Active() is true, the statusline displays the active
primary cursor index along with the total count:

  - Plain Statusline:
    VIS main.go [3/7 cursors]
  - Powerline Statusline:
    VIS │  main │ main.go │ [3/7] │ 14:8

This provides immediate situational awareness when selections exist outside the
visible viewport.

6. Edge Cases & Implementation Invariants

1.  Reverse Deletion Order: All multi-point buffer modifications (insertions and
    deletions) must process cursors in descending line and character order (for
    i := len(m.Cursors) - 1; i >= 0; i--).
2.  Buffer Traversal Wrap-Around: searchAndAdd wraps from EOF back to line 0,
    scanning lines modulo totalLines.
3.  Duplicate Prevention: searchAndAdd verifies that candidate positions do not
    overlap with any existing CursorInstance.Selection before creating a new
    cursor.
4.  Buffer Teardown: When a buffer is closed (CmdKillBuffer), buf.MultiCursor
    must be cleared to prevent stale memory references.
5.  Viewport Anchoring & Viewport Stability: The active window cursor
    (ContextCursorGet(ctx)) must remain anchored to m.Cursors[m.PrimaryIndex].
    Transitions between visual, insert, and normal modes must never hardcode
    index 0 or len - 1 directly, preventing viewport jumping when selections
    exist outside the visible screen.
6.  Stable Primary Index Tracking Across Sorting: Whenever m.Sort() reorders
    m.Cursors, it captures the line and column of m.Cursors[m.PrimaryIndex]
    beforehand and updates m.PrimaryIndex to match the new slice position of
    that cursor.


