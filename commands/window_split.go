package commands

import (
	"github.com/firstrow/wig"
)

const MaxWindowsPerPanel = 4
const MaxPanels = 2

// countWindowsInPanel counts how many currently open windows belong to the
// given panel (logical column). Panel membership now lives directly on
// wig.Window (set by wig.CmdWindowVSplit / wig.CmdWindowHSplit), so no
// separate side-table is needed here anymore.
func countWindowsInPanel(ctx wig.Context, panel int) int {
	count := 0
	for _, w := range ctx.Editor.Windows() {
		if w.Panel == panel {
			count++
		}
	}
	return count
}

func CmdWindowVSplitLimited(ctx wig.Context) {
	activeWin := ctx.Editor.ActiveWindow()

	if activeWin.Panel >= MaxPanels-1 {
		ctx.Editor.EchoMessage("Max 2 columns (left/right) reached")
		return
	}

	if len(ctx.Editor.Windows()) >= MaxWindowsPerPanel*MaxPanels {
		ctx.Editor.EchoMessage("Window limit reached (max 8)")
		return
	}

	wig.CmdWindowVSplit(ctx)
}

func CmdWindowHSplitLimited(ctx wig.Context) {
	activeWin := ctx.Editor.ActiveWindow()

	if countWindowsInPanel(ctx, activeWin.Panel) >= MaxWindowsPerPanel {
		ctx.Editor.EchoMessage("Window limit reached for this panel (max 4 up/down)")
		return
	}

	if len(ctx.Editor.Windows()) >= MaxWindowsPerPanel*MaxPanels {
		ctx.Editor.EchoMessage("Window limit reached (max 8)")
		return
	}

	wig.CmdWindowHSplit(ctx)
}
