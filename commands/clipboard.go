package commands

import (
	"github.com/atotto/clipboard"
	"github.com/firstrow/wig"
)

func CmdClipboardCopy(ctx wig.Context) {
	sel := wig.SelectionToString(ctx.Buf, ctx.Buf.Selection)
	if sel == "" {
		return
	}
	clipboard.WriteAll(sel)
	wig.CmdNormalMode(ctx)
	ctx.Editor.EchoMessage("copy to clipboard")
}

func CmdClipboardPaste(ctx wig.Context) {
	text, _ := clipboard.ReadAll()
	cur := wig.ContextCursorGet(ctx)

	if ctx.Buf.Selection != nil {
		if ctx.Buf.TxStart() {
			if ctx.Buf.Mode() == wig.MODE_VISUAL {
				wig.SelectionDelete(ctx)
			}
			if ctx.Buf.Mode() == wig.MODE_VISUAL_LINE {
				wig.SelectionDelete(ctx)
				line := wig.CursorLine(ctx.Buf, cur)
				wig.TextInsert(ctx.Buf, line, len(line.Value)-1, "\n")
			}
			ctx.Buf.TxEnd()
		}
	}

	if ctx.Buf.TxStart() {
		wig.TextInsert(ctx.Buf, wig.CursorLine(ctx.Buf, cur), cur.Char, text)
		ctx.Buf.TxEnd()
	}
	wig.CmdNormalMode(ctx)
}

func CmdClipboardPasteAll(ctx wig.Context) {
	text, err := clipboard.ReadAll()
	if err != nil {
		ctx.Editor.EchoMessage("clipboard read error: " + err.Error())
		return
	}
	wig.ReloadBufferContent(ctx, text)
	cur := wig.ContextCursorGet(ctx)
	cur.Line = 0
	cur.Char = 0
	cur.PreserveCharPosition = 0
	wig.CmdNormalMode(ctx)
}
