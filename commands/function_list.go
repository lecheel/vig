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
