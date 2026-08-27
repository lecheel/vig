package wig

// mark_adjust_locked mirrors vim's :lockmarks guard. While it is > 0, every
// range-adjustment entry point below returns immediately without touching
// any marks. Use LockMarksPush/LockMarksPop to bracket an edit that should
// not disturb mark positions.
var mark_adjust_locked int

// LockMarksPush increments the lock counter, disabling the mark adjustment
// functions below for the duration of the guarded edit.
func LockMarksPush() {
	mark_adjust_locked++
}

// LockMarksPop decrements the lock counter. It never goes below zero.
func LockMarksPop() {
	if mark_adjust_locked > 0 {
		mark_adjust_locked--
	}
}

// isSoftMark reports whether r is a lowercase a-z mark, which is allowed to
// collapse to a boundary rather than being invalidated when its text is
// deleted.
func isSoftMark(r rune) bool {
	return r >= 'a' && r <= 'z'
}

// isHardMark reports whether r is an uppercase A-Z or numeric 0-9 mark,
// which is invalidated (rather than relocated) when its text is deleted.
func isHardMark(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// MarkAdjustInternal updates global marks associated with buf to account
// for a text change affecting the closed interval [line1, line2]. It
// follows the same three-way classification vim's mark_adjust() uses:
//
//   - Before (lnum < line1):            unchanged.
//   - Inside (line1 <= lnum <= line2):  lnum + amount, unless amount < 0
//     (a deletion), in which case soft (lowercase) marks collapse onto
//     line1 and hard (uppercase/numeric) marks are invalidated.
//   - After  (lnum > line2):            lnum + amount + amount_after.
func MarkAdjustInternal(buf *Buffer, line1, line2 int, amount, amountAfter int) {
	if mark_adjust_locked > 0 {
		return
	}
	if EditorInst == nil || buf == nil || EditorInst.Marks == nil {
		return
	}
	for r, mark := range EditorInst.Marks {
		if mark.Buf != buf {
			continue
		}
		lnum := mark.Cursor.Line
		if lnum < 0 {
			continue
		}
		switch {
		case lnum < line1:
			continue
		case lnum <= line2:
			if amount < 0 {
				if isSoftMark(r) {
					mark.Cursor.Line = line1
					EditorInst.Marks[r] = mark
				} else if isHardMark(r) {
					delete(EditorInst.Marks, r)
				} else {
					mark.Cursor.Line = line1
					EditorInst.Marks[r] = mark
				}
			} else {
				mark.Cursor.Line = lnum + amount
				EditorInst.Marks[r] = mark
			}
		default:
			mark.Cursor.Line = lnum + amount + amountAfter
			EditorInst.Marks[r] = mark
		}
	}
}

// MarkColAdjust shifts the column of any mark on buf sitting exactly on
// lnum whose column is >= mincol, by nchars + ncharsAfter. This is for
// point insertions (text pushed right from a single column), as opposed to
// MarkColDelete below which handles a [start,end) range. A no-op while
// mark_adjust_locked > 0.
func MarkColAdjust(buf *Buffer, lnum, mincol int, nchars, ncharsAfter int) {
	if mark_adjust_locked > 0 {
		return
	}
	if EditorInst == nil || buf == nil || EditorInst.Marks == nil {
		return
	}
	for r, mark := range EditorInst.Marks {
		if mark.Buf != buf || mark.Cursor.Line != lnum {
			continue
		}
		if mark.Cursor.Char >= mincol {
			mark.Cursor.Char += nchars + ncharsAfter
			EditorInst.Marks[r] = mark
		}
	}
}

// MarkColDelete is the column-range counterpart of MarkAdjustInternal for a
// same-line deletion of [startCol, endCol) on lnum: marks before startCol
// are untouched, marks at/after endCol shift left by (endCol - startCol),
// and marks inside the deleted range collapse onto startCol (soft marks)
// or are invalidated (hard marks). A no-op while mark_adjust_locked > 0.
func MarkColDelete(buf *Buffer, lnum, startCol, endCol int) {
	if mark_adjust_locked > 0 {
		return
	}
	if EditorInst == nil || buf == nil || EditorInst.Marks == nil || endCol <= startCol {
		return
	}
	width := endCol - startCol
	for r, mark := range EditorInst.Marks {
		if mark.Buf != buf || mark.Cursor.Line != lnum {
			continue
		}
		switch {
		case mark.Cursor.Char < startCol:
			continue
		case mark.Cursor.Char < endCol:
			if isSoftMark(r) {
				mark.Cursor.Char = startCol
				EditorInst.Marks[r] = mark
			} else if isHardMark(r) {
				delete(EditorInst.Marks, r)
			} else {
				mark.Cursor.Char = startCol
				EditorInst.Marks[r] = mark
			}
		default:
			mark.Cursor.Char -= width
			EditorInst.Marks[r] = mark
		}
	}
}
