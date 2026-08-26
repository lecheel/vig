package wig

import (
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

type Transaction struct {
	buf    *Buffer
	before string
}

func NewTx(b *Buffer) *Transaction {
	return &Transaction{
		buf: b,
	}
}

func (tx *Transaction) Start() {
	tx.before = tx.buf.String()
}

func (tx *Transaction) End() {
	if tx.before == tx.buf.String() {
		return
	}

	apply := myers.ComputeEdits(span.URIFromPath("a.txt"), tx.before, tx.buf.String())
	undo := myers.ComputeEdits(span.URIFromPath("b.txt"), tx.buf.String(), tx.before)
	tx.buf.UndoRedo.Push(EditDiff{
		apply: apply,
		undo:  undo,
	})

	if len(apply) > 0 {
		if tx.buf.Highlighter != nil {
			tx.buf.Highlighter.Build()
		}
	}

	tx.buf = nil
	tx.before = ""
}

type EditDiff struct {
	apply []gotextdiff.TextEdit
	undo  []gotextdiff.TextEdit
}

type UndoRedo struct {
	Buf      *Buffer
	History  []EditDiff
	Position int
	// SavedAtPosition stores the UndoRedo Position at the moment the file was saved.
	// If Position == SavedAtPosition, the buffer is not dirty.
	SavedAtPosition int
}

func NewUndoRedo(buf *Buffer) *UndoRedo {
	return &UndoRedo{
		Buf:             buf,
		Position:        -1,
		SavedAtPosition: -1, // Initial state is considered "saved"
		History:         make([]EditDiff, 0, 256),
	}
}

func (u *UndoRedo) checkPosition() bool {
	if u.History == nil {
		return false
	}
	if u.Position > len(u.History) || u.Position < 0 {
		return false
	}

	return true
}

func (u *UndoRedo) Push(edits EditDiff) {
	if len(edits.apply) > 0 || len(edits.undo) > 0 {
		// we are back in history. remove all "forward" edits
		if u.Position >= 0 || u.Position != len(u.History)-1 {
			u.History = u.History[:u.Position+1]
		}

		u.History = append(u.History, edits)
		u.Position = len(u.History) - 1
	}
}

func (u *UndoRedo) Undo() {
	if !u.checkPosition() || u.Position < 0 {
		return
	}

	edits := u.History[u.Position].undo
	if len(edits) == 0 {
		return
	}

	u.applyMarksAdjust(edits)

	res := gotextdiff.ApplyEdits(u.Buf.String(), edits)
	u.Buf.ResetLines()
	u.Buf.Append(res)

	if u.Position >= 0 {
		u.Position--
	}

	// If we undid back to the exact state when the file was saved, it's not dirty
	u.Buf.Dirty = u.Position != u.SavedAtPosition

	if u.Buf.Highlighter != nil {
		u.Buf.Highlighter.Build()
	}
}

// applyMarksAdjust updates the marks in all windows showing this buffer
// based on the edits being applied. Because gotextdiff edits are absolute
// and non-overlapping, processing them in reverse (bottom-to-top) avoids
// line-number interference between multiple edits in the same transaction.
func (u *UndoRedo) applyMarksAdjust(edits []gotextdiff.TextEdit) {
	if EditorInst == nil {
		return
	}
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		// gotextdiff/span Positions are 1-based (token.Position convention);
		// wig's marks and MarkAdjustInternal use 0-based line numbers.
		startLine := e.Span.Start().Line() - 1
		endLine := e.Span.End().Line() - 1
		newText := e.NewText

		// Delete phase
		linesDeleted := endLine - startLine
		if linesDeleted > 0 {
			for _, w := range EditorInst.WindowsForBuffer(u.Buf) {
				MarkAdjustInternal(w, startLine, endLine, -linesDeleted, 0)
			}
		}

		// Insert phase
		newLines := strings.Count(newText, "\n")
		if newLines > 0 {
			for _, w := range EditorInst.WindowsForBuffer(u.Buf) {
				MarkAdjustInternal(w, startLine+1, startLine, newLines, 0)
			}
		}
	}
}

func (u *UndoRedo) Redo() {
	if !u.checkPosition() {
		return
	}

	edits := u.History[u.Position].apply
	if len(edits) == 0 {
		return
	}

	u.applyMarksAdjust(edits)

	res := gotextdiff.ApplyEdits(u.Buf.String(), edits)
	u.Buf.ResetLines()
	u.Buf.Append(res)

	if u.Position < len(u.History)-1 {
		u.Position++
	}

	// If we redid back to the exact state when the file was saved, it's not dirty
	u.Buf.Dirty = u.Position != u.SavedAtPosition

	if u.Buf.Highlighter != nil {
		u.Buf.Highlighter.Build()
	}
}
