package commands

import (
	"fmt"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func CmdFunctionList(ctx wig.Context) {
	buf := ctx.Buf
	if buf == nil {
		return
	}

	items := []ui.PickerItem[wig.Location]{}

	if buf.Highlighter == nil {
		ctx.Editor.EchoMessage("No functions found")
		return
	}

	// Try tree sitter
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		locations := ts.ListFunctions()
		for _, loc := range locations {
			items = append(items, ui.PickerItem[wig.Location]{
				Name:  fmt.Sprintf("%d: %s", loc.Line+1, loc.Text),
				Value: loc,
			})
		}
	}

	if len(items) == 0 {
		ctx.Editor.EchoMessage("No functions found")
		return
	}

	picker := ui.PickerInit(
		ctx.Editor,
		func(p *ui.UiPicker[wig.Location], i *ui.PickerItem[wig.Location]) {
			defer ctx.Editor.PopUi()
			if i == nil {
				return
			}
			ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, wig.ContextCursorGet(ctx))
			ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
				Line: i.Value.Line,
				Char: 0,
			})
			wig.CmdCursorCenter(ctx)
		},
		items,
	)
	picker.SetTitle("Functions")
}

// CmdGotoNextFunction jumps to the next function definition from the current line.
func CmdGotoNextFunction(ctx wig.Context) {
	gotoFunction(ctx, 1)
	ctx.Editor.LastRepeatableFn = CmdGotoNextFunction
}

// CmdGotoPrevFunction jumps to the previous function definition from the current line.
func CmdGotoPrevFunction(ctx wig.Context) {
	gotoFunction(ctx, -1)
	ctx.Editor.LastRepeatableFn = CmdGotoPrevFunction
}

func gotoFunction(ctx wig.Context, direction int) {
	if ctx.Buf == nil || ctx.Buf.Highlighter == nil {
		return
	}

	ts, ok := ctx.Buf.Highlighter.(*wig.TreeSitterHighlighter)
	if !ok || ts == nil {
		return
	}

	locations := ts.ListFunctions()
	if len(locations) == 0 {
		ctx.Editor.EchoMessage("No functions found")
		return
	}

	cur := wig.ContextCursorGet(ctx)
	var target *wig.Location

	if direction > 0 {
		// Find first function strictly after the current line
		for i := range locations {
			if locations[i].Line > cur.Line {
				target = &locations[i]
				break
			}
		}
		if target == nil {
			// Wrap around to the first function
			target = &locations[0]
			ctx.Editor.EchoMessage("search hit BOTTOM, continuing at TOP")
		}
	} else {
		// Find last function strictly before the current line
		for i := len(locations) - 1; i >= 0; i-- {
			if locations[i].Line < cur.Line {
				target = &locations[i]
				break
			}
		}
		if target == nil {
			// Wrap around to the last function
			target = &locations[len(locations)-1]
			ctx.Editor.EchoMessage("search hit TOP, continuing at BOTTOM")
		}
	}

	if target != nil {
		ctx.Editor.ActiveWindow().Jumps.Push(ctx.Buf, cur)
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{
			Line: target.Line,
			Char: 0,
		})
		wig.CmdCursorCenter(ctx)
	}
}
