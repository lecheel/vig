package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/mattn/go-runewidth"
)

func StatuslineRender(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
) {
	buf := win.Buffer()
	if buf == nil {
		return
	}

	w, h := view.Size()
	h -= 1

	st := wig.Color("ui.statusline.inactive")

	if win == e.ActiveWindow() {
		st = wig.Color("ui.statusline")

		if !e.Config.SameStatuslineColor && buf.Mode() == wig.MODE_INSERT {
			st = wig.Color("ui.statusline.insert")
		}
	}

	bg := strings.Repeat(" ", w)
	if h >= 0 {
		view.SetContent(0, h, bg, st)
	}

	macroStatus := ""
	if e.Keys.Macros.Recording() {
		macroStatus = "recording @" + e.Keys.Macros.Register
	}

	leftSide := fmt.Sprintf("%s %s %s ", buf.Mode().String(), buf.GetName(), macroStatus)

	if (win == e.ActiveWindow() || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		leftSide = e.Message
	}

	if h >= 0 {
		view.SetContent(2, h, leftSide, st)
	}

	cur := wig.CursorGet(e, buf)
	wsIndicator := "🔒"
	if e.Config.SaveWorkspaces {
		wsIndicator = "💾"
	}

	// Show the innermost function containing the cursor line, beside the
	// workspace indicator on the right side of the statusline. Uses the
	// same tree-sitter query as CmdFunctionList (F8) — via
	// TreeSitterHighlighter.FunctionAtLine — so the name shown matches
	// what the F8 picker would list for this buffer. Empty for non-TS
	// buffers (no Highlighter, or a non-tree-sitter Highlighter like the
	// git diff / quickfix / health ones) or when the cursor is outside
	// any function (imports, package decl, top-level constants, etc.).
	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}

	rightSide := fmt.Sprintf("%s[workspace: %d] %d:%d", wsIndicator, e.ActiveWorkspace, cur.Line+1, cur.Char)
	if funcName != "" {
		// Cap function name length so very long Go method names
		// (e.g. TestSomeVeryLongScenario_Subcase_ABC) don't push the
		// workspace indicator off-screen on narrow terminals.
		if len(funcName) > 30 {
			funcName = funcName[:27] + "..."
		}
		rightSide = fmt.Sprintf("%s  %s", funcName, rightSide)
	}

	if e.Keys.GetCount() > 1 {
		rightSide = fmt.Sprintf("%d   %s", e.Keys.GetCount(), rightSide)
	}

	visWidth := runewidth.StringWidth(rightSide)
	if w-visWidth-1 >= 0 && h >= 0 {
		view.SetContent(w-visWidth-1, h, rightSide, st)
	}
}
