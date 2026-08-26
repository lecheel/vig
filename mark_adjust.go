package wig

// mark_adjust_locked mirrors vim's :lockmarks guard. While it is > 0, every
// range-adjustment entry point below returns immediately without touching
// any marks. Use LockMarksPush/LockMarksPop to bracket an edit that should
// not disturb mark positions.
var mark_adjust_locked int

// LockMarksPush increments the lock counter, disabling MarkAdjustInternal
// and MarkColAdjust for the duration of the guarded edit.
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
// slide to line1 rather than being invalidated when its line is deleted.
func isSoftMark(r rune) bool {
	return r >= 'a' && r <= 'z'
}

// isHardMark reports whether r is an uppercase A-Z or numeric 0-9 mark,
// which is invalidated (rather than relocated) when its line is deleted.
func isHardMark(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// MarkAdjustInternal updates every mark stored on win to account for a text
// change affecting the closed interval [line1, line2]. It follows the same
// three-way classification vim's mark_adjust() uses:
//
//   - Before (lnum < line1):            unchanged.
//   - Inside (line1 <= lnum <= line2):  lnum + amount, unless amount < 0
//     (a deletion), in which case soft (lowercase) marks collapse onto
//     line1 and hard (uppercase/numeric) marks are invalidated.
//   - After  (lnum > line2):            lnum + amount + amount_after.
//
// amount is the net number of lines inserted (positive) or deleted
// (negative) strictly inside [line1, line2]. amount_after is an additional
// shift applied only to marks after line2 (e.g. lines appended by :put
// beyond the affected range).
//
// All range logic is skipped while mark_adjust_locked > 0 (:lockmarks).
func MarkAdjustInternal(win *Window, line1, line2 int, amount, amountAfter int) {
	if mark_adjust_locked > 0 {
		return
	}
	if win == nil || win.Marks == nil {
		return
	}

	for r, cur := range win.Marks {
		lnum := cur.Line

		// A mark with an invalid position is skipped entirely.
		if lnum < 0 {
			continue
		}

		switch {
		case lnum < line1:
			// Before: range starts after the mark. Unchanged.
			continue

		case lnum <= line2:
			// Inside [line1, line2].
			if amount < 0 {
				// Deletion inside the range (includes the "entire range
				// wiped" case where amount == -(line2-line1+1)).
				if isSoftMark(r) {
					cur.Line = line1
					win.Marks[r] = cur
				} else if isHardMark(r) {
					// Hard marks die rather than move. There is no
					// natural lnum==0 sentinel in wig's 0-based line
					// numbering, so an invalidated hard mark is removed
					// from the map outright.
					delete(win.Marks, r)
				} else {
					// Unknown mark kind: default to the safer "move to
					// line1" behavior rather than silently discarding it.
					cur.Line = line1
					win.Marks[r] = cur
				}
			} else {
				cur.Line = lnum + amount
				win.Marks[r] = cur
			}

		default:
			// After (lnum > line2): shifted by the total delta.
			cur.Line = lnum + amount + amountAfter
			win.Marks[r] = cur
		}
	}
}

// MarkColAdjust adjusts the column of any mark sitting exactly on lnum.
// Marks whose column is >= mincol shift by nchars + ncharsAfter; marks with
// a smaller column are left untouched. Like MarkAdjustInternal, this is a
// no-op while mark_adjust_locked > 0.
func MarkColAdjust(win *Window, lnum, mincol int, nchars, ncharsAfter int) {
	if mark_adjust_locked > 0 {
		return
	}
	if win == nil || win.Marks == nil {
		return
	}

	for r, cur := range win.Marks {
		if cur.Line != lnum {
			continue
		}
		if cur.Char >= mincol {
			cur.Char += nchars + ncharsAfter
			win.Marks[r] = cur
		}
	}
}
