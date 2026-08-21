package commands

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func CmdExit(ctx wig.Context) {
	checkDirtyAndExit(ctx, 0)
}

func CmdForceExit(ctx wig.Context) {
	ctx.Editor.ExitCh <- 1
}

func checkDirtyAndExit(ctx wig.Context, startIndex int) {
	buffers := ctx.Editor.Buffers
	for i := startIndex; i < len(buffers); i++ {
		b := buffers[i]
		if b.FilePath != "" && !strings.HasPrefix(b.FilePath, "[") && b.Dirty {
			prompt := fmt.Sprintf("Save changes to \"%s\"? (y/n/c)", b.GetName())
			ui.ConfirmInit(ctx, prompt, func() {
				if err := b.Save(); err != nil {
					ctx.Editor.EchoMessage(err.Error())
					return
				}
				checkDirtyAndExit(ctx, i+1)
			}, func() {
				checkDirtyAndExit(ctx, i+1)
			}, func() {
				ctx.Editor.EchoMessage("Exit cancelled")
			})
			return
		}
	}
	ctx.Editor.ExitCh <- 1
}
