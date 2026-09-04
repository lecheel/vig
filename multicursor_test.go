package wig

import (
	"testing"

	"github.com/firstrow/wig/testutils"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiCursorMatchNext(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1

	CmdMultiCursorMatchNext(ctx)
	require.True(t, buf.MultiCursor.Active())
	require.Equal(t, 2, buf.MultiCursor.Count())
	require.Equal(t, "foo", buf.MultiCursor.Pattern)
	require.Equal(t, MODE_VISUAL, buf.Mode())

	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())

	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
}

func TestMultiCursorChange(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0

	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())

	CmdSelectionChange(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())

	for _, ch := range "test" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}

	CmdNormalMode(ctx)

	expected := "test bar test\nbaz foo\n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorDelete(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0

	CmdMultiCursorMatchNext(ctx)
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())

	CmdSelectionDelete(ctx)
	require.Equal(t, MODE_NORMAL, buf.Mode())

	expected := " bar \nbaz \n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorBackspace(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("word1 word2")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}
	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0
	ctx.Buf.Selection = &Selection{
		Start: Cursor{Line: 0, Char: 0},
		End:   Cursor{Line: 0, Char: 3},
	}
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())
	CmdSelectionChange(ctx)
	for _, ch := range "abc" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	CmdNormalMode(ctx)
	expected := "ab1 ab2\n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorSkipNext(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}
	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())
	CmdMultiCursorSkipNext(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())
	require.Equal(t, 1, buf.MultiCursor.Cursors[1].Cursor.Line)
}

func TestMultiCursorMoveDownCollapsesSelection(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}
	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, MODE_VISUAL, buf.Mode())
	CmdCursorLineDown(ctx)
	require.Equal(t, MODE_NORMAL, buf.Mode())
	for _, c := range buf.MultiCursor.Cursors {
		require.Nil(t, c.Selection)
	}
}

func TestMultiCursorInsertAtEachCursor(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}
	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1
	CmdMultiCursorMatchNext(ctx)
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
	CmdEnterInsertMode(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())
	for _, ch := range "X" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	CmdNormalMode(ctx)
	expected := "Xfoo bar Xfoo\nbaz Xfoo\n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorRotation(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo bar foo\nbaz foo")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}
	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1

	CmdMultiCursorMatchNext(ctx)
	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
	require.Equal(t, 2, buf.MultiCursor.PrimaryIndex)

	// Rotate forward wraps to 0
	CmdMultiCursorRotateForward(ctx)
	require.Equal(t, 0, buf.MultiCursor.PrimaryIndex)
	require.Equal(t, 0, cur.Line)
	require.Equal(t, 2, cur.Char)

	// Rotate backward wraps back to 2
	CmdMultiCursorRotateBackward(ctx)
	require.Equal(t, 2, buf.MultiCursor.PrimaryIndex)
	require.Equal(t, 1, cur.Line)
}

func TestMultiCursorAddDown(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo\nbar\nbaz\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 1

	CmdMultiCursorAddDown(ctx)
	require.True(t, buf.MultiCursor.Active())
	require.Equal(t, 2, buf.MultiCursor.Count())
	require.Equal(t, 0, buf.MultiCursor.Cursors[0].Cursor.Line)
	require.Equal(t, 1, buf.MultiCursor.Cursors[0].Cursor.Char)
	require.Equal(t, 1, buf.MultiCursor.Cursors[1].Cursor.Line)
	require.Equal(t, 1, buf.MultiCursor.Cursors[1].Cursor.Char)

	CmdMultiCursorAddDown(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
	require.Equal(t, 2, buf.MultiCursor.Cursors[2].Cursor.Line)
	require.Equal(t, 1, buf.MultiCursor.Cursors[2].Cursor.Char)
}

func TestMultiCursorAppendEndOfLines(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("hello\nworld\nfoo\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0

	// Press C twice to have 3 cursors
	CmdMultiCursorAddDown(ctx)
	CmdMultiCursorAddDown(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())

	// Press A to append at end of lines
	CmdAppendEndOfLines(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())

	for _, ch := range "!" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	CmdNormalMode(ctx)

	expected := "hello!\nworld!\nfoo!\n"
	assert.Equal(t, expected, buf.String())
}

func TestVisualLineAppendEndOfLines(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("first\nsecond\nthird\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	// Select lines 0 to 2 in visual line mode
	buf.SetMode(MODE_VISUAL_LINE)
	buf.Selection = &Selection{
		Start: Cursor{Line: 0, Char: 0},
		End:   Cursor{Line: 2, Char: 5},
	}

	CmdAppendEndOfLines(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())
	require.Equal(t, 3, buf.MultiCursor.Count())

	for _, ch := range ";" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	CmdNormalMode(ctx)

	expected := "first;\nsecond;\nthird;\n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorAddDownAndChange(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo\nbar\nbaz\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0

	// Add 3 cursors/selections with C
	CmdMultiCursorAddDown(ctx)
	CmdMultiCursorAddDown(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
	require.Equal(t, MODE_VISUAL, buf.Mode())

	// Press 'c' to change and enter insert mode
	CmdSelectionChange(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())

	for _, ch := range "z" {
		HandleInsertKey(ctx, tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	CmdNormalMode(ctx)

	expected := "zoo\nzar\nzaz\n"
	assert.Equal(t, expected, buf.String())
}

func TestMultiCursorAddDownAndDelete(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("foo\nbar\nbaz\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0

	// Add 3 cursors/selections with C
	CmdMultiCursorAddDown(ctx)
	CmdMultiCursorAddDown(ctx)
	require.Equal(t, 3, buf.MultiCursor.Count())
	require.Equal(t, MODE_VISUAL, buf.Mode())

	// Press 'x' to delete the selected character
	CmdSelectionDelete(ctx)
	assert.Equal(t, "oo\nar\naz\n", buf.String())
	require.True(t, buf.MultiCursor.Active())
	require.Equal(t, 3, buf.MultiCursor.Count())
	require.Equal(t, MODE_NORMAL, buf.Mode())

	// Press 'x' in normal mode to delete next char under each cursor
	CmdDeleteCharForward(ctx)
	assert.Equal(t, "o\nr\nz\n", buf.String())
	require.Equal(t, 3, buf.MultiCursor.Count())
}

func TestVisualModeAddDown(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	buf.Append("abcd\nefgh\nijkl\n")
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 2
	buf.SetMode(MODE_VISUAL)
	buf.Selection = &Selection{
		Start: Cursor{Line: 0, Char: 1},
		End:   Cursor{Line: 0, Char: 2},
	}

	CmdMultiCursorAddDown(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())
	require.NotNil(t, buf.MultiCursor.Cursors[0].Selection)
	require.NotNil(t, buf.MultiCursor.Cursors[1].Selection)
	require.Equal(t, 1, buf.MultiCursor.Cursors[1].Selection.Start.Char)
	require.Equal(t, 2, buf.MultiCursor.Cursors[1].Selection.End.Char)
}

func TestMultiCursorInsertPreservesViewport(t *testing.T) {
	e := NewEditor(testutils.Viewport, nil)
	buf := NewBuffer()
	buf.ResetLines()
	for i := 0; i < 220; i++ {
		if i == 10 || i == 200 {
			buf.Append("foo bar\n")
		} else {
			buf.Append("line\n")
		}
	}
	e.ActiveWindow().ShowBuffer(buf)
	ctx := Context{
		Editor: e,
		Buf:    buf,
		Win:    e.ActiveWindow(),
	}

	cur := ContextCursorGet(ctx)
	cur.Line = 10
	cur.Char = 0

	CmdMultiCursorMatchNext(ctx)
	require.Equal(t, 2, buf.MultiCursor.Count())
	require.Equal(t, 1, buf.MultiCursor.PrimaryIndex)
	require.Equal(t, 200, cur.Line)

	savedOffset := cur.ScrollOffset

	// Pressing 'a' must remain on line 200 and not reset ScrollOffset to 0
	CmdEnterInsertModeAppend(ctx)
	require.Equal(t, MODE_INSERT, buf.Mode())
	require.Equal(t, 200, cur.Line)
	require.Equal(t, savedOffset, cur.ScrollOffset)
}
