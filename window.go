package wig

type Window struct {
	buf     *Buffer // active buffer
	cursors map[*Buffer]*Cursor
	Jumps   *Jumps
	// Panel is the logical column index this window belongs to (0 = left,
	// 1 = right, ...). Windows sharing a Panel are stacked vertically
	// within that column; distinct Panels are arranged left-to-right.
	// Set by CmdWindowVSplit (new column) / CmdWindowHSplit (same column)
	// in movements_cmds.go and consumed by render.Renderer.Render.
	Panel int
}

// Jump to buffer and location. Records jump history.
func (win *Window) VisitBuffer(ctx Context, cursor ...Cursor) {
	if ctx.Buf == nil {
		return
	}

	cur := WindowCursorGet(win, win.buf)
	if win.buf != nil {
		win.Jumps.Push(win.buf, cur)
	}

	if len(cursor) > 0 {
		newCur := &Cursor{}
		newCur.Line = cursor[0].Line
		newCur.Char = cursor[0].Char
		newCur.ScrollOffset = cursor[0].ScrollOffset
		win.cursors[ctx.Buf] = newCur
	}

	// Push the *destination* buffer's actual cursor position, not the
	// cursor we just left behind in the old buffer. Previously this
	// reused `cur` (the old buffer's line/char) paired with ctx.Buf,
	// producing a corrupted jump-list entry every time buffers switched.
	// That bogus entry is what made pingpong (CmdJumpToggle) and the
	// jump-back/jump-forward navigation land on the wrong line.
	destCur := WindowCursorGet(win, ctx.Buf)
	win.Jumps.Push(ctx.Buf, destCur)
	win.buf = ctx.Buf

	ctx.Win = win
	CmdCursorCenter(ctx)
}

// Show buffer. No history.
func (win *Window) ShowBuffer(buf *Buffer) {
	if buf != nil {
		win.buf = buf
	}
}

func (win *Window) Buffer() *Buffer {
	return win.buf
}

// Specify parent window to inherit cursors
func CreateWindow(parent *Window) *Window {
	cursors := map[*Buffer]*Cursor{}
	if parent != nil {
		for k, v := range parent.cursors {
			cursors[k] = &Cursor{
				Line:                 v.Line,
				Char:                 v.Char,
				PreserveCharPosition: v.PreserveCharPosition,
				ScrollOffset:         v.ScrollOffset,
			}
		}
	}

	return &Window{
		Jumps: &Jumps{
			List: List[Jump]{},
		},
		cursors: cursors,
	}
}

// Jumps
type Jump struct {
	FilePath string
	Cursor   Cursor
}

type Jumps struct {
	List    List[Jump]
	current *Element[Jump]
}

func (j *Jumps) Push(b *Buffer, cur *Cursor) {
	// Track jumps including character position, so that jumping to a
	// definition on the same line (e.g. from column 7 to column 5) is
	// recorded as a distinct jump and can be jumped back from.
	if j.List.Last() != nil {
		last := j.List.Last().Value
		if last.FilePath == b.FilePath && last.Cursor.Line == cur.Line && last.Cursor.Char == cur.Char {
			return
		}
	}
	j.List.PushBack(Jump{
		FilePath: b.FilePath,
		Cursor:   *cur,
	})
	j.current = j.List.Last()
}

func (j *Jumps) JumpBack() {
	elem := j.List.Last()
	if elem == nil {
		return
	}

	if j.current != nil && j.current != elem {
		elem = j.current
	}

	if elem.Prev() == nil {
		return
	}

	item := elem.Prev().Value
	b := EditorInst.BufferFindByFilePath(item.FilePath, false)
	if b == nil {
		return
	}

	cur := CursorGet(EditorInst, b)
	cur.Line = item.Cursor.Line
	cur.Char = item.Cursor.Char
	cur.ScrollOffset = item.Cursor.ScrollOffset

	EditorInst.ActiveWindow().buf = b
	j.current = elem.Prev()
}

func (j *Jumps) JumpForward() {
	if j.current == nil {
		return
	}

	item := j.current.Next()
	if item == nil {
		return
	}

	b := EditorInst.BufferFindByFilePath(item.Value.FilePath, false)
	if b == nil {
		return
	}
	cur := CursorGet(EditorInst, b)
	cur.Line = item.Value.Cursor.Line
	cur.Char = item.Value.Cursor.Char
	cur.ScrollOffset = item.Value.Cursor.ScrollOffset
	EditorInst.ActiveWindow().buf = b
	j.current = item
}
