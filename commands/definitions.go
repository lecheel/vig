package commands

import (
	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func init() {
	wig.AllCommands["CmdMiniHelp"] = wig.CmdDefinition{Desc: "Mini help", Fn: ui.CmdMiniHelp}
	wig.AllCommands["CmdFunctionList"] = wig.CmdDefinition{Desc: "Function list", Fn: CmdFunctionList}
	wig.AllCommands["CmdFormatBuffer"] = wig.CmdDefinition{Desc: "Format buffer", Fn: CmdFormatBuffer}
	wig.AllCommands["CmdSearchProject"] = wig.CmdDefinition{Desc: "Search project", Fn: CmdSearchProject}
	wig.AllCommands["CmdJumpForward"] = wig.CmdDefinition{Desc: "Jump forward", Fn: wig.CmdJumpForward}
	wig.AllCommands["CmdReloadBuffer"] = wig.CmdDefinition{Desc: "Reload buffer", Fn: CmdReloadBuffer}
	wig.AllCommands["CmdNewBuffer"] = wig.CmdDefinition{Desc: "New buffer", Fn: wig.CmdNewBuffer}
	wig.AllCommands["CmdExit"] = wig.CmdDefinition{Desc: "Quit editor", Fn: CmdExit}
	wig.AllCommands["CmdCommandPalettePicker"] = wig.CmdDefinition{Desc: "Command palette", Fn: CmdCommandPalettePicker}
	wig.AllCommands["CmdFindProjectFilePicker"] = wig.CmdDefinition{Desc: "Find file", Fn: CmdFindProjectFilePicker}
	wig.AllCommands["CmdBufferPicker"] = wig.CmdDefinition{Desc: "Buffer picker", Fn: CmdBufferPicker}
	wig.AllCommands["CmdBufferNext"] = wig.CmdDefinition{Desc: "Next buffer", Fn: wig.CmdBufferNext}
	wig.AllCommands["CmdBufferPrev"] = wig.CmdDefinition{Desc: "Previous buffer", Fn: wig.CmdBufferPrev}
	wig.AllCommands["CmdBufferLast"] = wig.CmdDefinition{Desc: "Last buffer", Fn: wig.CmdBufferLast}
	wig.AllCommands["CmdWindowNext"] = wig.CmdDefinition{Desc: "Next window", Fn: wig.CmdWindowNext}
	wig.AllCommands["CmdKillBuffer"] = wig.CmdDefinition{Desc: "Kill buffer", Fn: wig.CmdKillBuffer}
	wig.AllCommands["CmdSaveFile"] = wig.CmdDefinition{Desc: "Save file", Fn: CmdSaveFileWithFeedback}
	wig.AllCommands["CmdGitHunkRevert"] = wig.CmdDefinition{Desc: "Revert git hunk", Fn: CmdGitHunkRevert}
	wig.AllCommands["CmdSelectRegister"] = wig.CmdDefinition{Desc: "Select register", Fn: wig.CmdSelectRegister}
	wig.AllCommands["CmdInsertRegister"] = wig.CmdDefinition{Desc: "Insert register", Fn: wig.CmdInsertRegister}
	wig.AllCommands["CmdShowRegisters"] = wig.CmdDefinition{Desc: "Show registers", Fn: wig.CmdShowRegisters}
	wig.AllCommands["registers"] = wig.CmdDefinition{Desc: "Show registers", Fn: wig.CmdShowRegisters}
	wig.AllCommands["reg"] = wig.CmdDefinition{Desc: "Show registers", Fn: wig.CmdShowRegisters}
	wig.AllCommands["CmdDeleteLine"] = wig.CmdDefinition{Desc: "Delete line", Fn: wig.CmdDeleteLine}
	wig.AllCommands["CmdGitHunkPreview"] = wig.CmdDefinition{Desc: "Preview git hunk", Fn: CmdGitHunkPreview}
	wig.AllCommands["CmdMRUBufferPicker"] = wig.CmdDefinition{Desc: "MRU Buffer Picker", Fn: CmdMRUBufferPicker}
	wig.AllCommands["CmdCheckHealth"] = wig.CmdDefinition{Desc: "Check health of dependencies", Fn: CmdCheckHealth}
	wig.AllCommands["checkhealth"] = wig.CmdDefinition{Desc: "Check health of dependencies", Fn: CmdCheckHealth}
	wig.AllCommands["CmdInfo"] = wig.CmdDefinition{Desc: "Show notification info", Fn: CmdInfo}
	wig.AllCommands["info"] = wig.CmdDefinition{Desc: "Show notification info", Fn: CmdInfo}

	// Repeatable Ex commands (can be repeated with `.`)
	wig.AllCommands["cn"] = wig.CmdDefinition{Desc: "Next result", Fn: wig.CmdVisitNextLine, Repeatable: true}
	wig.AllCommands["cp"] = wig.CmdDefinition{Desc: "Previous result", Fn: wig.CmdVisitPrevLine, Repeatable: true}
	wig.AllCommands["CmdGitHunkNext"] = wig.CmdDefinition{Desc: "Next git hunk", Fn: CmdGitHunkNext, Repeatable: true}
	wig.AllCommands["CmdGitHunkPrev"] = wig.CmdDefinition{Desc: "Previous git hunk", Fn: CmdGitHunkPrev, Repeatable: true}

	// Command-line basics
	wig.AllCommands["q"] = wig.CmdDefinition{Desc: "Quit", Fn: CmdExit}
	wig.AllCommands["q!"] = wig.CmdDefinition{Desc: "Quit without saving", Fn: CmdForceExit}
	wig.AllCommands["w"] = wig.CmdDefinition{Desc: "Save", Fn: CmdSaveFileWithFeedback} // Enhanced feedback for :w
	wig.AllCommands["wq"] = wig.CmdDefinition{Desc: "Save and quit", Fn: func(ctx wig.Context) {
		CmdSaveFileWithFeedback(ctx) // Enhanced feedback for :wq
		CmdExit(ctx)
	}}
	wig.AllCommands["bd"] = wig.CmdDefinition{Desc: "Delete buffer", Fn: wig.CmdKillBuffer}
	wig.AllCommands["bn"] = wig.CmdDefinition{Desc: "Next buffer", Fn: wig.CmdBufferNext}
	wig.AllCommands["bp"] = wig.CmdDefinition{Desc: "Previous buffer", Fn: wig.CmdBufferPrev}
	wig.AllCommands["bl"] = wig.CmdDefinition{Desc: "Last buffer", Fn: wig.CmdBufferLast}
	wig.AllCommands["CmdGitView"] = wig.CmdDefinition{Desc: "Git status panel", Fn: CmdGitView}
	wig.AllCommands["gs"] = wig.CmdDefinition{Desc: "Git status", Fn: CmdGitView}
	wig.AllCommands["CmdGitFilesPicker"] = wig.CmdDefinition{Desc: "Git changed files", Fn: CmdGitFilesPicker}
	wig.AllCommands["CmdGitStatusFilePicker"] = wig.CmdDefinition{Desc: "Git changed files", Fn: CmdGitFilesPicker}
	wig.AllCommands["gfiles"] = wig.CmdDefinition{Desc: "Git changed files", Fn: CmdGitFilesPicker}
	wig.AllCommands["CmdGitBlame"] = wig.CmdDefinition{Desc: "Git blame", Fn: CmdGitBlame}
	wig.AllCommands["blame"] = wig.CmdDefinition{Desc: "Git blame", Fn: CmdGitBlame}
	wig.AllCommands["CmdGitBlameCommit"] = wig.CmdDefinition{Desc: "Git blame commit detail", Fn: CmdGitBlameCommit}

	// Window management commands
	wig.AllCommands["vs"] = wig.CmdDefinition{Desc: "Vertical split", Fn: wig.CmdWindowVSplit}
	wig.AllCommands["sp"] = wig.CmdDefinition{Desc: "Horizontal split", Fn: wig.CmdWindowVSplit} // wig only has VSplit for now
	wig.AllCommands["on"] = wig.CmdDefinition{Desc: "Close other windows", Fn: wig.CmdWindowCloseOther}
	wig.AllCommands["only"] = wig.CmdDefinition{Desc: "Close other windows", Fn: wig.CmdWindowCloseOther}
	wig.AllCommands["close"] = wig.CmdDefinition{Desc: "Close window", Fn: wig.CmdWindowClose}

	// Additional commands used in default keymap
	wig.AllCommands["CmdExecute"] = wig.CmdDefinition{Desc: "Execute buffer", Fn: CmdExecute}
	wig.AllCommands["CmdCurrentBufferDirFilePicker"] = wig.CmdDefinition{Desc: "Find file in current dir", Fn: CmdCurrentBufferDirFilePicker}
	wig.AllCommands["CmdGotoDefinition"] = wig.CmdDefinition{Desc: "Go to definition", Fn: CmdGotoDefinition}
	wig.AllCommands["CmdGotoDefinitionOtherWindow"] = wig.CmdDefinition{Desc: "Go to definition in other window", Fn: CmdGotoDefinitionOtherWindow}
	wig.AllCommands["CmdViewDefinitionOtherWindow"] = wig.CmdDefinition{Desc: "View definition in other window", Fn: CmdViewDefinitionOtherWindow}
	wig.AllCommands["CmdLspShowSignature"] = wig.CmdDefinition{Desc: "Show LSP signature", Fn: CmdLspShowSignature}
	wig.AllCommands["CmdLspHover"] = wig.CmdDefinition{Desc: "LSP hover", Fn: CmdLspHover}
	wig.AllCommands["CmdLspShowDiagnostics"] = wig.CmdDefinition{Desc: "Show LSP diagnostics", Fn: CmdLspShowDiagnostics}
	wig.AllCommands["CmdSearchLine"] = wig.CmdDefinition{Desc: "Search line in buffer", Fn: CmdSearchLine}
	wig.AllCommands["CmdThemeSelect"] = wig.CmdDefinition{Desc: "Select theme", Fn: CmdThemeSelect}
	wig.AllCommands["CmdClipboardCopy"] = wig.CmdDefinition{Desc: "Copy to clipboard", Fn: CmdClipboardCopy}
	wig.AllCommands["CmdClipboardPaste"] = wig.CmdDefinition{Desc: "Paste from clipboard", Fn: CmdClipboardPaste}
	wig.AllCommands["CmdProjectSearchWordUnderCursor"] = wig.CmdDefinition{Desc: "Search project for word under cursor", Fn: CmdProjectSearchWordUnderCursor}
	wig.AllCommands["CmdGotoFile"] = wig.CmdDefinition{Desc: "Go to file under cursor", Fn: wig.CmdGotoFile}
	wig.AllCommands["CmdGotoFileOtherWindow"] = wig.CmdDefinition{Desc: "Go to file under cursor in other window", Fn: wig.CmdGotoFileOtherWindow}
	wig.AllCommands["CmdToggleComment"] = wig.CmdDefinition{Desc: "Toggle comment", Fn: wig.CmdToggleComment}
	wig.AllCommands["CmdCommentLine"] = wig.CmdDefinition{Desc: "Comment line", Fn: wig.CmdCommentLine, Repeatable: true}
	wig.AllCommands["gcc"] = wig.CmdDefinition{Desc: "Comment line", Fn: wig.CmdCommentLine, Repeatable: true}
	wig.AllCommands["CmdCommentLineDown"] = wig.CmdDefinition{Desc: "Comment line down", Fn: wig.CmdCommentLineDown, Repeatable: true}
	wig.AllCommands["CmdCommentLineUp"] = wig.CmdDefinition{Desc: "Comment line up", Fn: wig.CmdCommentLineUp, Repeatable: true}
	wig.AllCommands["CmdCommentEndOfLine"] = wig.CmdDefinition{Desc: "Comment to end of line", Fn: wig.CmdCommentEndOfLine, Repeatable: true}
	wig.AllCommands["CmdCommentParagraph"] = wig.CmdDefinition{Desc: "Comment paragraph", Fn: wig.CmdCommentParagraph, Repeatable: true}
	wig.AllCommands["CmdCommentAroundParagraph"] = wig.CmdDefinition{Desc: "Comment around paragraph", Fn: wig.CmdCommentAroundParagraph, Repeatable: true}
	wig.AllCommands["CmdCommentEndOfFile"] = wig.CmdDefinition{Desc: "Comment to end of file", Fn: wig.CmdCommentEndOfFile, Repeatable: true}
	wig.AllCommands["CmdCommentStartOfFile"] = wig.CmdDefinition{Desc: "Comment to start of file", Fn: wig.CmdCommentStartOfFile, Repeatable: true}
	wig.AllCommands["CmdCommentWord"] = wig.CmdDefinition{Desc: "Comment word", Fn: wig.CmdCommentWord, Repeatable: true}
	wig.AllCommands["CmdWindowVSplit"] = wig.CmdDefinition{Desc: "Vertical split", Fn: wig.CmdWindowVSplit}
	wig.AllCommands["CmdWindowClose"] = wig.CmdDefinition{Desc: "Close window", Fn: wig.CmdWindowClose}
	wig.AllCommands["CmdWindowCloseOther"] = wig.CmdDefinition{Desc: "Close other windows", Fn: wig.CmdWindowCloseOther}
	wig.AllCommands["CmdWindowCloseAndKillBuffer"] = wig.CmdDefinition{Desc: "Close window and kill buffer", Fn: wig.CmdWindowCloseAndKillBuffer}
	wig.AllCommands["CmdWindowToggleLayout"] = wig.CmdDefinition{Desc: "Toggle window layout", Fn: wig.CmdWindowToggleLayout}
	wig.AllCommands["CmdJumpBack"] = wig.CmdDefinition{Desc: "Jump back", Fn: wig.CmdJumpBack}
	wig.AllCommands["CmdAutocompleteTrigger"] = wig.CmdDefinition{Desc: "Trigger autocomplete", Fn: wig.CmdAutocompleteTrigger}
	wig.AllCommands["CmdBufferCycle"] = wig.CmdDefinition{Desc: "Cycle buffers", Fn: wig.CmdBufferCycle}
	wig.AllCommands["CmdSearchWordUnderCursor"] = wig.CmdDefinition{Desc: "Search word under cursor", Fn: CmdSearchWordUnderCursor}
	wig.AllCommands["CmdFormatBufferAndSave"] = wig.CmdDefinition{Desc: "Format buffer and save", Fn: CmdFormatBufferAndSave}
	wig.AllCommands["CmdMakeBuild"] = wig.CmdDefinition{Desc: "Make build", Fn: CmdMakeBuild}
	wig.AllCommands["CmdMakeTest"] = wig.CmdDefinition{Desc: "Make test", Fn: CmdMakeTest}
	wig.AllCommands["CmdRgUnderCursor"] = wig.CmdDefinition{Desc: "Ripgrep search under cursor", Fn: CmdRgUnderCursor}
	wig.AllCommands["CmdOpenSavedSearch"] = wig.CmdDefinition{Desc: "Reopen rg search", Fn: CmdOpenSavedSearch}
	wig.AllCommands["copen"] = wig.CmdDefinition{Desc: "Open quickfix list", Fn: CmdQuickfixOpen}
	wig.AllCommands["cclose"] = wig.CmdDefinition{Desc: "Close quickfix window", Fn: func(ctx wig.Context) {
		if len(ctx.Editor.Windows) > 1 {
			wig.CmdWindowClose(ctx)
		} else {
			wig.CmdBufferCycle(ctx)
		}
	}}
	wig.AllCommands["CmdDummyNA"] = wig.CmdDefinition{Desc: "Disable key", Fn: wig.CmdDummyNA}
	wig.AllCommands["nop"] = wig.CmdDefinition{Desc: "NOP", Fn: wig.CmdDummyNA}
}
