package wig

import (
	"cmp"
	"context"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	odin "github.com/firstrow/tree-sitter-odin/bindings/go"
	ts_toml "github.com/tree-sitter-grammars/tree-sitter-toml/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
	bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	clang "github.com/tree-sitter/tree-sitter-c/bindings/go"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
	ts_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
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
	if h == nil || h.tree == nil || event.Buf == nil {
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

func detectShebang(buf *Buffer) string {
	if buf == nil || buf.Lines.Len == 0 {
		return ""
	}
	firstLineNode := buf.Lines.First()
	if firstLineNode == nil {
		return ""
	}

	firstLine := strings.TrimSpace(firstLineNode.Value.String())
	if !strings.HasPrefix(firstLine, "#!") {
		return ""
	}

	lower := strings.ToLower(firstLine)
	switch {
	case strings.Contains(lower, "python"):
		return "python"
	case strings.Contains(lower, "bash"), strings.Contains(lower, "sh"), strings.Contains(lower, "zsh"):
		return "bash"
	}
	return ""
}

func TreeSitterHighlighterInitBuffer(e *Editor, buf *Buffer) *TreeSitterHighlighter {
	var treeSitterLang unsafe.Pointer
	qpath := ""

	shebang := detectShebang(buf)

	base := filepath.Base(buf.FilePath)

	switch {
	case strings.HasSuffix(buf.FilePath, ".go"):
		treeSitterLang = golang.Language()
		qpath = "go"
	case strings.HasSuffix(buf.FilePath, ".odin"):
		treeSitterLang = odin.Language()
		qpath = "odin"
	case strings.HasSuffix(buf.FilePath, ".c"), strings.HasSuffix(buf.FilePath, ".h"):
		treeSitterLang = clang.Language()
		qpath = "c"
	case strings.HasSuffix(buf.FilePath, ".py") || shebang == "python":
		treeSitterLang = python.Language()
		qpath = "python"
	case strings.HasSuffix(buf.FilePath, ".rs"):
		treeSitterLang = rust.Language()
		qpath = "rust"
	case strings.HasSuffix(buf.FilePath, ".json"), strings.HasSuffix(buf.FilePath, ".jsonc"):
		treeSitterLang = ts_json.Language()
		qpath = "json"
	case strings.HasSuffix(buf.FilePath, ".toml"):
		treeSitterLang = ts_toml.Language()
		qpath = "toml"
	case strings.HasSuffix(buf.FilePath, ".sh"), strings.HasSuffix(buf.FilePath, ".bash"), strings.HasSuffix(buf.FilePath, ".zsh"),
		base == ".bashrc" || base == ".bash_profile" || base == ".zshrc" || shebang == "bash":
		treeSitterLang = bash.Language()
		qpath = "bash"
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

func NewHighlighterForPath(buf *Buffer, path string) Highlighter {
	if EditorInst == nil {
		return nil
	}
	hl := TreeSitterHighlighterInitBuffer(EditorInst, buf)
	if hl == nil {
		return nil
	}
	return hl
}

func (h *TreeSitterHighlighter) HighlightLine(lineNum int) []Span {
	tslock.Lock()
	defer tslock.Unlock()

	if h == nil || h.tree == nil || h.q == nil {
		return nil
	}

	qc := sitter.NewQueryCursor()
	qc.SetPointRange(
		sitter.Point{Row: uint(lineNum), Column: 0},
		sitter.Point{Row: uint(lineNum + 1), Column: 0},
	)
	defer qc.Close()

	matches := qc.Matches(h.q, h.tree.RootNode(), h.sourceCode)

	var spans []Span
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			startPos := capture.Node.StartPosition()
			endPos := capture.Node.EndPosition()

			if int(startPos.Row) > lineNum || int(endPos.Row) < lineNum {
				continue
			}

			startCol := uint16(0)
			if int(startPos.Row) == lineNum {
				startCol = uint16(startPos.Column)
			}

			endCol := uint16(math.MaxUint16)
			if int(endPos.Row) == lineNum {
				endCol = uint16(endPos.Column)
			}

			nodeName := h.q.CaptureNames()[capture.Index]
			style := Color(nodeName)

			spans = append(spans, Span{
				StartCol: startCol,
				EndCol:   endCol,
				Style:    style,
			})
		}
	}

	slices.SortFunc(spans, func(a, b Span) int {
		if a.StartCol != b.StartCol {
			return cmp.Compare(a.StartCol, b.StartCol)
		}
		return cmp.Compare(a.EndCol, b.EndCol)
	})

	return spans
}

func (h *TreeSitterHighlighter) ListFunctions() []Location {
	tslock.Lock()
	defer tslock.Unlock()

	if h.tree == nil {
		return nil
	}

	var queryStr string
	var treeSitterLang unsafe.Pointer

	shebang := detectShebang(h.buf)
	base := filepath.Base(h.buf.FilePath)

	switch {
	case strings.HasSuffix(h.buf.FilePath, ".go"):
		queryStr = "(function_declaration name: (identifier) @name) (method_declaration name: (field_identifier) @name)"
		treeSitterLang = golang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".rs"):
		queryStr = "(function_item name: (identifier) @name)"
		treeSitterLang = rust.Language()
	case strings.HasSuffix(h.buf.FilePath, ".py") || shebang == "python":
		queryStr = "(function_definition name: (identifier) @name)"
		treeSitterLang = python.Language()
	case strings.HasSuffix(h.buf.FilePath, ".c"), strings.HasSuffix(h.buf.FilePath, ".h"):
		queryStr = "(function_definition declarator: (function_declarator declarator: (identifier) @name))"
		treeSitterLang = clang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".sh"), strings.HasSuffix(h.buf.FilePath, ".bash"), strings.HasSuffix(h.buf.FilePath, ".zsh"),
		base == ".bashrc" || base == ".bash_profile" || base == ".zshrc" || shebang == "bash":
		queryStr = "(function_definition name: (word) @name)"
		treeSitterLang = bash.Language()
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

// FunctionAtLine returns the name of the innermost function containing the
// given 0-indexed line, or "" if the line is outside any function (e.g.
// imports, package decl, module-level constants). It uses the same
// per-language tree-sitter queries as ListFunctions (the data behind the
// F8 picker) but, instead of returning every function, walks each match's
// captured name node up to its parent function declaration and picks the
// smallest enclosing range. The statusline calls this on every render so
// the user can see "you are inside func Foo" beside the workspace
// indicator without opening the picker.
//
// Why parent and not the captured node itself: the @name capture is on
// the identifier/field_identifier (the function's name token), not the
// function_declaration node, so the name node's range is just the span
// of the identifier — typically one row. The parent is the
// function_declaration/method_declaration whose StartRow..EndRow is the
// full function body, which is what we need to test containment.
func (h *TreeSitterHighlighter) FunctionAtLine(lineNum int) string {
	tslock.Lock()
	defer tslock.Unlock()

	if h == nil || h.tree == nil {
		return ""
	}

	var queryStr string
	var treeSitterLang unsafe.Pointer

	shebang := detectShebang(h.buf)
	base := filepath.Base(h.buf.FilePath)

	switch {
	case strings.HasSuffix(h.buf.FilePath, ".go"):
		queryStr = "(function_declaration name: (identifier) @name) (method_declaration name: (field_identifier) @name)"
		treeSitterLang = golang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".rs"):
		queryStr = "(function_item name: (identifier) @name)"
		treeSitterLang = rust.Language()
	case strings.HasSuffix(h.buf.FilePath, ".py") || shebang == "python":
		queryStr = "(function_definition name: (identifier) @name)"
		treeSitterLang = python.Language()
	case strings.HasSuffix(h.buf.FilePath, ".c"), strings.HasSuffix(h.buf.FilePath, ".h"):
		queryStr = "(function_definition declarator: (function_declarator declarator: (identifier) @name))"
		treeSitterLang = clang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".sh"), strings.HasSuffix(h.buf.FilePath, ".bash"), strings.HasSuffix(h.buf.FilePath, ".zsh"),
		base == ".bashrc" || base == ".bash_profile" || base == ".zshrc" || shebang == "bash":
		queryStr = "(function_definition name: (word) @name)"
		treeSitterLang = bash.Language()
	default:
		return ""
	}

	q, err := sitter.NewQuery(sitter.NewLanguage(treeSitterLang), queryStr)
	if err != nil {
		return ""
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	matches := qc.Matches(q, h.tree.RootNode(), h.sourceCode)

	var bestName string
	bestRange := math.MaxInt
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, cap := range match.Captures {
			nameNode := cap.Node
			parent := nameNode.Parent()
			pStart := int(parent.StartPosition().Row)
			pEnd := int(parent.EndPosition().Row)

			if lineNum >= pStart && lineNum <= pEnd {
				// Innermost function wins — smallest range containing
				// the line. Matters for languages with nested functions
				// (Python nested defs, inner Rust blocks); for Go's
				// top-level funcs it just picks the only containing one.
				rangeSize := pEnd - pStart
				if rangeSize < bestRange {
					bestRange = rangeSize
					bestName = string(h.sourceCode[nameNode.StartByte():nameNode.EndByte()])
				}
			}
		}
	}

	return bestName
}

func (h *TreeSitterHighlighter) FunctionRange(lineNum int) (startLine, endLine int, found bool) {
	tslock.Lock()
	defer tslock.Unlock()
	if h == nil || h.tree == nil {
		return 0, 0, false
	}
	var queryStr string
	var treeSitterLang unsafe.Pointer
	shebang := detectShebang(h.buf)
	base := filepath.Base(h.buf.FilePath)
	switch {
	case strings.HasSuffix(h.buf.FilePath, ".go"):
		queryStr = "(function_declaration name: (identifier) @name) (method_declaration name: (field_identifier) @name)"
		treeSitterLang = golang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".rs"):
		queryStr = "(function_item name: (identifier) @name)"
		treeSitterLang = rust.Language()
	case strings.HasSuffix(h.buf.FilePath, ".py") || shebang == "python":
		queryStr = "(function_definition name: (identifier) @name)"
		treeSitterLang = python.Language()
	case strings.HasSuffix(h.buf.FilePath, ".c"), strings.HasSuffix(h.buf.FilePath, ".h"):
		queryStr = "(function_definition declarator: (function_declarator declarator: (identifier) @name))"
		treeSitterLang = clang.Language()
	case strings.HasSuffix(h.buf.FilePath, ".sh"), strings.HasSuffix(h.buf.FilePath, ".bash"), strings.HasSuffix(h.buf.FilePath, ".zsh"),
		base == ".bashrc" || base == ".bash_profile" || base == ".zshrc" || shebang == "bash":
		queryStr = "(function_definition name: (word) @name)"
		treeSitterLang = bash.Language()
	default:
		return 0, 0, false
	}
	q, err := sitter.NewQuery(sitter.NewLanguage(treeSitterLang), queryStr)
	if err != nil {
		return 0, 0, false
	}
	defer q.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	matches := qc.Matches(q, h.tree.RootNode(), h.sourceCode)
	bestRange := math.MaxInt
	found = false
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, cap := range match.Captures {
			nameNode := cap.Node
			parent := nameNode.Parent()
			pStart := int(parent.StartPosition().Row)
			pEnd := int(parent.EndPosition().Row)
			if lineNum >= pStart && lineNum <= pEnd {
				rangeSize := pEnd - pStart
				if rangeSize < bestRange {
					bestRange = rangeSize
					startLine = pStart
					endLine = pEnd
					found = true
				}
			}
		}
	}
	return startLine, endLine, found
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
