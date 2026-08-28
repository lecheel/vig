Here is a concise, implementation-ready Mini Specification for the :sp and :vs
split window subsystem in wig.

Mini Spec: :sp / :vs Split Window Subsystem

1. Goal

Provide a nested, recursive split-window model supporting arbitrary vertical
(:vs) and horizontal (:sp) splits, isolated viewport rendering, accurate
per-pane scrolling math, and consistent window lifecycle management.

2. Data Structures

2.1 Split Tree (WinNode)

type SplitDir int

const (
    SplitNone SplitDir = iota // Leaf: holds a *Window
    SplitVertical             // Children arranged side-by-side (left/right)
    SplitHorizontal           // Children stacked (top/bottom)
)

type WinNode struct {
    Dir      SplitDir
    Win      *Window    // Non-nil only when Dir == SplitNone
    Children []*WinNode // Exactly 2 children for binary splits, or N for flattened splits
}

2.2 Window Dimensions

Each Window tracks its rendered layout bounds so movement and scrolling math use
localized pane dimensions:

type Window struct {
    // ... existing fields ...
    X, Y, W, H int // Computed during each render pass
}

3. Core Commands & Logic

3.1 Split Operation (:sp, :vs)

  - Trigger: :vs / CmdWindowVSplit, :sp / CmdWindowHSplit.
  - Behavior:
    1.  Find the *WinNode leaf corresponding to ctx.Editor.ActiveWindow().
    2.  Create a new window nwin := CreateWindow(activeWin) sharing buffer and
        cursor position.
    3.  Turn the active leaf into a split node:
          - leaf.Dir = SplitVertical (for :vs) or SplitHorizontal (for :sp).
          - leaf.Children = []*WinNode{leafNode(activeWin), leafNode(nwin)}.
          - leaf.Win = nil.
    4.  Append nwin to ctx.Editor.Windows.
    5.  Set active window to nwin.

3.2 Close Window (:q, CmdWindowClose)

  - Behavior:
    1.  If len(ctx.Editor.Windows) == 1, refuse to close (or quit editor if
        configured).
    2.  Remove target window's leaf from ctx.Editor.Root via removeLeaf.
    3.  Collapse any intermediary node with only 1 child to that child.
    4.  Focus the nearest adjacent leaf in the tree.
    5.  Remove the window from ctx.Editor.Windows.

3.3 Close Other Windows (:only, CmdWindowCloseOther)

  - Behavior:
    1.  Retain only activeWin.
    2.  ctx.Editor.Root = leafNode(activeWin).
    3.  Clear all other windows from ctx.Editor.Windows.

4. Layout & Rendering Algorithm

4.1 Separator & Geometry Calculation

Split geometry must allocate explicit single-cell separators rather than drawing
over child viewports.

  - Vertical Split (SplitVertical):
      - Total separator cells: numSeps = len(Children) - 1.
      - Available width for panes: availW = W - numSeps.
      - Base child width: baseW = availW / len(Children).
      - For child i:
          - \text{width}_i = \text{baseW} + (\text{remainder if } i == \text{last}).
          - Separator drawn at X_{\text{sep}} = X_i - 1 using tcell.RuneVLine
            (for i > 0).
  - Horizontal Split (SplitHorizontal):
      - Total separator cells: numSeps = len(Children) - 1.
      - Available height for panes: availH = H - numSeps.
      - Base child height: baseH = availH / len(Children).
      - For child i:
          - \text{height}_i = \text{baseH} + (\text{remainder if } i == \text{last}).
          - Separator drawn at Y_{\text{sep}} = Y_i - 1 using tcell.RuneHLine
            (for i > 0).

4.2 Leaf Viewport Render

For each leaf (Dir == SplitNone):

1.  Update node.Win.X, node.Win.Y, node.Win.W, node.Win.H.
2.  Construct sub-view mview := NewMView(screen, x, y, w, h).
3.  Invoke ui.WindowRender(editor, mview, node.Win).
4.  If enabled, invoke RenderIndentGuides(node.Win, mview).
5.  Invoke ui.StatuslineRender(editor, mview, node.Win) (occupies bottom row h
    - 1).

5. Scrolling & Movement Geometry

viewHeight and viewWidth in movements_cmds.go must reflect the active window's
actual dimensions:

func viewHeight(ctx Context) int {
    win := ctx.Editor.ActiveWindow()
    if win != nil && win.H > 0 {
        return win.H - 1 // Reserve 1 line for statusline
    }
    _, h := ctx.Editor.View.Size()
    return h - 1
}

  - Ensures CmdScrollUpPage, CmdScrollDownPage, CmdEnsureCursorVisible, and
    CmdCursorCenter calculate offsets against the split pane bounds instead of
    the global terminal size.

6. Window Traversal & Navigation

  - Tree Order Cycling (CmdWindowNext / CmdWindowPrev):
      - Traversal follows depth-first in-order leaf order from Root
        (left-to-right, top-to-bottom) rather than flat creation slice order.
  - Directional Navigation (Ctrl-w h/j/k/l) (Extension):
      - Finds the closest window whose bounding box (X, Y, W, H) is adjacent in
        the specified direction.
