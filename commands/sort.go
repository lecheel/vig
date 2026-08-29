package commands

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/firstrow/wig"
)

var sortNumberRe = regexp.MustCompile(`-?\d+`)

// sortFirstNumber returns the first integer found in s, or 0 if none.
// Used by the numeric-sort flag (n) — mirrors Vim's ":sort n" semantics
// where lines without a number are treated as 0.
func sortFirstNumber(s string) int {
	m := sortNumberRe.FindString(s)
	if m == "" {
		return 0
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0
	}
	return n
}

// CmdSort sorts the lines in the buffer (or the current selection).
//
// Flags (passed via ctx.Char, e.g. `:sort un`):
//
//	u — remove duplicate lines after sorting
//	r — reverse the sort order
//	n — sort numerically by the first number on each line
//	i — ignore case when comparing
//
// Without a selection the whole buffer is sorted; with a visual /
// visual-line / visual-block selection only the covered lines are sorted.
// The change is recorded as a single undo/redo transaction.
func CmdSort(ctx wig.Context) {
	buf := ctx.Buf
	if buf == nil {
		return
	}

	flags := strings.TrimSpace(ctx.Char)
	unique := strings.Contains(flags, "u")
	reverse := strings.Contains(flags, "r")
	numeric := strings.Contains(flags, "n")
	ignoreCase := strings.Contains(flags, "i")

	startLine := 0
	endLine := buf.Lines.Len - 1

	if buf.Selection != nil {
		sel := wig.SelectionNormalize(buf.Selection)
		startLine = sel.Start.Line
		endLine = sel.End.Line
	}

	if startLine < 0 {
		startLine = 0
	}
	if endLine >= buf.Lines.Len {
		endLine = buf.Lines.Len - 1
	}
	if endLine-startLine < 1 {
		return
	}

	// Collect the text content of each line (without the trailing newline).
	lines := make([]string, 0, endLine-startLine+1)
	for i := startLine; i <= endLine; i++ {
		lineNode := wig.CursorLineByNum(buf, i)
		if lineNode == nil {
			break
		}
		lines = append(lines, strings.TrimSuffix(string(lineNode.Value), "\n"))
	}

	if len(lines) < 2 {
		return
	}

	// Build the comparison function. Stable sort preserves the original
	// relative order of equal elements, matching Vim's default behaviour.
	less := func(i, j int) bool {
		a, b := lines[i], lines[j]
		if ignoreCase {
			a = strings.ToLower(a)
			b = strings.ToLower(b)
		}
		if numeric {
			return sortFirstNumber(a) < sortFirstNumber(b)
		}
		return a < b
	}

	sort.SliceStable(lines, less)

	if reverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	if unique {
		deduped := lines[:0]
		for i, l := range lines {
			if i == 0 || l != lines[i-1] {
				deduped = append(deduped, l)
			}
		}
		lines = deduped
	}

	// Apply the change transactionally: delete the original line range
	// (including the trailing newline of the last line) and insert the
	// sorted text in its place. This mirrors the pattern used by
	// revertHunk in commands/git_hunk.go.
	if buf.TxStart() {
		defer buf.TxEnd()
	}

	endLineNode := wig.CursorLineByNum(buf, endLine)
	if endLineNode == nil {
		return
	}
	endChar := len(endLineNode.Value) - 1 // index of the trailing '\n'

	wig.TextDelete(buf, &wig.Selection{
		Start: wig.Cursor{Line: startLine, Char: 0},
		End:   wig.Cursor{Line: endLine, Char: endChar + 1},
	})

	newText := strings.Join(lines, "\n") + "\n"

	startLineNode := wig.CursorLineByNum(buf, startLine)
	if startLineNode == nil {
		// The deleted range ended at the end of the buffer; append to the
		// new last line.
		lastLine := buf.Lines.Last()
		if lastLine != nil {
			wig.TextInsert(buf, lastLine, len(lastLine.Value)-1, newText)
		}
	} else {
		wig.TextInsert(buf, startLineNode, 0, newText)
	}

	// Drop the selection so the editor returns to a clean normal-mode
	// state, matching how Vim leaves you after `:'<,'>sort`.
	buf.Selection = nil

	cur := wig.ContextCursorGet(ctx)
	if cur != nil {
		cur.Line = startLine
		cur.Char = 0
		cur.PreserveCharPosition = 0
	}
	wig.CmdNormalMode(ctx)
	wig.CmdCursorCenter(ctx)
	ctx.Editor.EchoMessage("sorted")
}
