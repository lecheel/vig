package wig

import (
	"sync"
	"testing"

	"github.com/firstrow/wig/testutils"
	"github.com/stretchr/testify/require"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// TODO: rewrite/fix treesitter concurrency tests

func TestTreeSitterHighlightLine(t *testing.T) {
	source := `package wig

import "fmt"

func add(a int, b int) {
	fmt.Printf("%d", a+b)
}`

	e := NewEditor(
		testutils.Viewport,
		nil,
	)
	buf := e.BufferFindByFilePath("testfile.go", true)
	buf.ResetLines()
	buf.Append(source)
	buf.Highlighter = TreeSitterHighlighterInitBuffer(e, buf)
	require.NotNil(t, buf.Highlighter)

	spans := buf.Highlighter.HighlightLine(0)
	require.NotEmpty(t, spans)
}

func TestTreeSitter_AdaptEventTextChange(t *testing.T) {
	source := `package wig

import "fmt"

func add(a int, b int) {
	fmt.Printf("%d", a+b)
}`

	e := NewEditor(
		testutils.Viewport,
		nil,
	)
	buf := e.BufferFindByFilePath("testfile.go", true)
	buf.ResetLines()
	buf.Append(source)
	require.Equal(t, source+"\n", buf.String())
	buf.Highlighter = TreeSitterHighlighterInitBuffer(e, buf)
	highlighter := buf.Highlighter.(*TreeSitterHighlighter)

	events := e.Events.Subscribe()
	defer e.Events.Unsubscribe(events)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := <-events
		msg.Wg.Done()
		event := msg.Msg.(EventTextChange)
		require.Equal(t, EventTextChange{
			Buf:     buf,
			Start:   Position{Line: 4, Char: 5},
			End:     Position{Line: 4, Char: 8},
			Text:    "",
			OldText: "add",
		}, event)

		actual := highlighter.editEditInput(event)
		expected := sitter.InputEdit{
			StartPosition:  sitter.Point{Row: 4, Column: 5},
			OldEndPosition: sitter.Point{Row: 4, Column: 8},
			NewEndPosition: sitter.Point{Row: 4, Column: 5},
			StartByte:      uint(32),
			OldEndByte:     uint(35),
			NewEndByte:     uint(32),
		}
		require.Equal(t, expected, actual)
	}()

	line := CursorLineByNum(buf, 4)
	TextDelete(buf, &Selection{
		Start: Cursor{Line: 4, Char: 5},
		End:   Cursor{Line: 4, Char: 8},
	})
	require.Equal(t, "func (a int, b int) {\n", line.Value.String())
	wg.Wait()

}

func TestTreeSitter_AdaptEventTextChangeDeleteLine(t *testing.T) {
	source := `package wig

import "fmt"

func add(a int, b int) {
	fmt.Printf("%d", a+b)
}`

	e := NewEditor(
		testutils.Viewport,
		nil,
	)

	buf := e.BufferFindByFilePath("testfile.go", true)
	buf.ResetLines()
	buf.Append(source)
	cur := CursorGet(e, buf)
	cur.Line = 4
	cur.Char = 0
	require.Equal(t, source+"\n", buf.String())
	buf.Highlighter = TreeSitterHighlighterInitBuffer(e, buf)
	highlighter := buf.Highlighter.(*TreeSitterHighlighter)

	events := e.Events.Subscribe()
	defer e.Events.Unsubscribe(events)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := <-events
		msg.Wg.Done()
		event := msg.Msg.(EventTextChange)
		require.Equal(t, EventTextChange{
			Buf:     buf,
			Start:   Position{Line: 4, Char: 0},
			End:     Position{Line: 5, Char: 0},
			Text:    "",
			OldText: "func add(a int, b int) {\n",
		}, event)

		expected := sitter.InputEdit{
			StartPosition:  sitter.Point{Row: 4, Column: 0},
			OldEndPosition: sitter.Point{Row: 5, Column: 0},
			NewEndPosition: sitter.Point{Row: 4, Column: 0},
			StartByte:      uint(27),
			OldEndByte:     uint(52),
			NewEndByte:     uint(27),
		}

		actual := highlighter.editEditInput(event)
		require.Equal(t, expected, actual)
	}()

	CmdDeleteLine(Context{
		Editor: e,
		Buf:    buf,
		Count:  0,
		Char:   "",
	})
	wg.Wait()
}
