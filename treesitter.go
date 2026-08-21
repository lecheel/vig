package wig

import (
	"context"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	odin "github.com/firstrow/tree-sitter-odin/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
	clang "github.com/tree-sitter/tree-sitter-c/bindings/go"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

// TODO: rewrite treesitter to use channel and scheduled parsing. some day.
var tslock sync.Mutex

var _ Highlighter = &TreeSitterHighlighter{}

type TreeSitterHighlighter struct {
	e          *Editor
	buf        *Buffer
	parser     *sitter.Parser
	q          *sitter.Query
	tree       *sitter.Tree
	sourceCode []byte
}

func TreeSitterHighlighterGo(e *Editor) {
	go func() {
		for event := range e.Events.Subscribe() {
			switch e := event.Msg.(type) {
			case EventTextChange:
				if e.Buf.Highlighter != nil {
					e.Buf.Highlighter.TextChanged(e)
				}
			}
			event.Wg.Done()
		}
	}()
}

func (h *TreeSitterHighlighter) TextChanged(event EventTextChange) {
	if event.Buf == nil {
		return
	}

	tslock.Lock()
	defer tslock.Unlock()

	ll := h.editEditInput(event)
	h.tree.Edit(&ll)

	h.sourceCode = []byte(event.Buf.String())
	tree := h.parser.ParseCtx(context.Background(), h.sourceCode, h.tree)
	h.tree.Close()
	h.tree = tree
}

func TreeSitterHighlighterInitBuffer(e *Editor, buf *Buffer) *TreeSitterHighlighter {
	var treeSitterLang unsafe.Pointer
	qpath := ""

	switch {
	case strings.HasSuffix(buf.FilePath, ".go"):
		treeSitterLang = golang.Language()
		qpath = "go"

	case strings.HasSuffix(buf.FilePath, ".odin"):
		treeSitterLang = odin.Language()
		qpath = "odin"
	case strings.HasSuffix(buf.FilePath, ".c"):
		treeSitterLang = clang.Language()
		qpath = "c"
	case strings.HasSuffix(buf.FilePath, ".h"):
		treeSitterLang = clang.Language()
		qpath = "c"
	case strings.HasSuffix(buf.FilePath, ".py"):
		treeSitterLang = python.Language()
		qpath = "python"
	case strings.HasSuffix(buf.FilePath, ".rs"):
		treeSitterLang = rust.Language()
		qpath = "rust"
	default:
		return nil
	}

	h := &TreeSitterHighlighter{
		e:   e,
		buf: buf,
	}
	h.parser = sitter.NewParser()
	h.parser.SetLanguage(sitter.NewLanguage(treeSitterLang))
	var err error

	hgFile := e.RuntimeDir("queries", qpath, "highlights.scm")

	highlightQ, err := os.ReadFile(hgFile)
	if err != nil {
		EditorInst.LogError(err, true)
		return nil
	}

	// TODO: check that weird error. geeeeeee.
	h.q, _ = sitter.NewQuery(sitter.NewLanguage(treeSitterLang), string(highlightQ))

	h.Build()
	return h
}

func (h *TreeSitterHighlighter) Build() {
	tslock.Lock()
	defer tslock.Unlock()

	if h == nil {
		return
	}

	if h.tree != nil {
		h.tree.Close()
	}

	h.sourceCode = []byte(h.buf.String())
	tree := h.parser.Parse(h.sourceCode, nil)
	h.tree = tree
}

func (h *TreeSitterHighlighter) ForRange(lineStart, lineEnd uint32) *HighlighterCursor {
	tslock.Lock()
	defer tslock.Unlock()

	qc := sitter.NewQueryCursor()
	qc.SetPointRange(
		sitter.Point{Row: uint(lineStart), Column: 0},
		sitter.Point{Row: uint(lineEnd), Column: 0},
	)
	defer qc.Close()

	matches := qc.Matches(h.q, h.tree.RootNode(), h.sourceCode)

	nodes := List[HighlighterNode]{}

	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			row := capture.Node.StartPosition().Row
			col := capture.Node.StartPosition().Column
			erow := capture.Node.EndPosition().Row
			ecol := capture.Node.EndPosition().Column
			nodes.PushBack(HighlighterNode{
				NodeName:  h.q.CaptureNames()[capture.Index],
				StartLine: uint32(row),
				StartChar: uint32(col),
				EndLine:   uint32(erow),
				EndChar:   uint32(ecol),
			})
		}
	}

	return &HighlighterCursor{
		Cursor: nodes.First(),
	}
}

func (h *TreeSitterHighlighter) ListFunctions() []Location {
	tslock.Lock()
	defer tslock.Unlock()

	if h.tree == nil {
		return nil
	}

	var queryStr string
	var treeSitterLang unsafe.Pointer

	switch {
	case strings.HasSuffix(h.buf.FilePath, ".go"):
		queryStr = "(function_declaration name: (identifier) @name) (method_declaration name: (field_identifier) @name)"
		treeSitterLang = golang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".rs"):
		queryStr = "(function_item name: (identifier) @name)"
		treeSitterLang = rust.Language()
	case strings.HasSuffix(h.buf.FilePath, ".py"):
		queryStr = "(function_definition name: (identifier) @name)"
		treeSitterLang = python.Language()
	case strings.HasSuffix(h.buf.FilePath, ".c"), strings.HasSuffix(h.buf.FilePath, ".h"):
		queryStr = "(function_definition declarator: (function_declarator declarator: (identifier) @name))"
		treeSitterLang = clang.Language()
	default:
		return nil
	}

	q, err := sitter.NewQuery(sitter.NewLanguage(treeSitterLang), queryStr)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	matches := qc.Matches(q, h.tree.RootNode(), h.sourceCode)

	var funcs []Location
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, cap := range match.Captures {
			node := cap.Node
			startByte := node.StartByte()
			endByte := node.EndByte()
			text := string(h.sourceCode[startByte:endByte])
			funcs = append(funcs, Location{
				Text: text,
				Line: int(node.StartPosition().Row),
				Char: 0,
			})
		}
	}

	return funcs
}

func (h *TreeSitterHighlighter) editEditInput(event EventTextChange) (r sitter.InputEdit) {
	pointToByte := func(buf *Buffer, line, char int) int {
		size := 0
		lineNum := 0
		currentLine := buf.Lines.First()
		for currentLine != nil {
			if lineNum == line {
				v := currentLine.Value.Range(0, char)
				return size + utf8.RuneCountInString(string(v))
			}
			size += currentLine.Value.Bytes()
			currentLine = currentLine.Next()
			lineNum++
		}
		return size
	}

	// deletion
	if len(event.Text) == 0 {
		return sitter.InputEdit{
			StartPosition:  sitter.Point{Row: uint(event.Start.Line), Column: uint(event.Start.Char)},
			OldEndPosition: sitter.Point{Row: uint(event.End.Line), Column: uint(event.End.Char)},
			NewEndPosition: sitter.Point{Row: uint(event.Start.Line), Column: uint(event.Start.Char)},
			StartByte:      uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char)),
			OldEndByte:     uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char) + len(event.OldText)),
			NewEndByte:     uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char)),
		}
	}

	// insertion
	return sitter.InputEdit{
		StartPosition:  sitter.Point{Row: uint(event.Start.Line), Column: uint(event.Start.Char)},
		OldEndPosition: sitter.Point{Row: uint(event.Start.Line), Column: uint(event.Start.Char)},
		NewEndPosition: sitter.Point{Row: uint(event.NewEnd.Line), Column: uint(event.NewEnd.Char)},
		StartByte:      uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char)),
		OldEndByte:     uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char)),
		NewEndByte:     uint(pointToByte(event.Buf, event.Start.Line, event.Start.Char) + utf8.RuneCountInString(event.Text)),
	}
}
