package commands

import (
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func init() {
	wig.AllCommands["CmdMiniHelp"] = wig.CmdDefinition{Desc: "Mini help", Fn: ui.CmdMiniHelp}
	wig.AllCommands["CmdFunctionList"] = wig.CmdDefinition{Desc: "Function list", Fn: CmdFunctionList}
	wig.AllCommands["CmdGotoNextFunction"] = wig.CmdDefinition{Desc: "Next function", Fn: CmdGotoNextFunction, Repeatable: true}
	wig.AllCommands["CmdGotoPrevFunction"] = wig.CmdDefinition{Desc: "Previous function", Fn: CmdGotoPrevFunction, Repeatable: true}
	wig.AllCommands["CmdFormatBuffer"] = wig.CmdDefinition{Desc: "Format buffer", Fn: CmdFormatBuffer}
	wig.AllCommands["CmdSearchProject"] = wig.CmdDefinition{Desc: "Search project", Fn: CmdSearchProject}
	wig.AllCommands["CmdJumpForward"] = wig.CmdDefinition{Desc: "Jump forward", Fn: wig.CmdJumpForward}
	wig.AllCommands["CmdReloadBuffer"] = wig.CmdDefinition{Desc: "Reload buffer", Fn: CmdReloadBuffer}
	wig.AllCommands["CmdNewBuffer"] = wig.CmdDefinition{Desc: "New buffer", Fn: wig.CmdNewBuffer}
	wig.AllCommands["CmdExit"] = wig.CmdDefinition{Desc: "Quit editor", Fn: CmdExit}
	wig.AllCommands["CmdCommandPalettePicker"] = wig.CmdDefinition{Desc: "Command palette", Fn: CmdCommandPalettePicker}
	wig.AllCommands["CmdHelp"] = wig.CmdDefinition{Desc: "Show command help palette", Fn: CmdHelp}
	wig.AllCommands["help"] = wig.CmdDefinition{Desc: "Show command help palette", Fn: CmdHelp}
	wig.AllCommands["CmdFindProjectFilePicker"] = wig.CmdDefinition{Desc: "Find file", Fn: CmdFindProjectFilePicker}
	wig.AllCommands["CmdBufferPicker"] = wig.CmdDefinition{Desc: "Buffer picker", Fn: CmdBufferPicker}
	wig.AllCommands["CmdMarksPicker"] = wig.CmdDefinition{Desc: "Marks picker (global, cross-buffer)", Fn: wig.CmdGotoMark}
	wig.AllCommands["CmdGotoMark"] = wig.CmdDefinition{Desc: "Marks picker (global, cross-buffer)", Fn: wig.CmdGotoMark}
	wig.AllCommands["marks"] = wig.CmdDefinition{Desc: "Marks picker (global, cross-buffer)", Fn: wig.CmdGotoMark}
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
	wig.AllCommands["CmdToggleBool"] = wig.CmdDefinition{Desc: "Toggle boolean under cursor", Fn: CmdToggleBool}
	wig.AllCommands["toggle"] = wig.CmdDefinition{Desc: "Toggle boolean under cursor", Fn: CmdToggleBool}
	wig.AllCommands["sort"] = wig.CmdDefinition{Desc: "Sort lines (flags: u n r i)", Fn: CmdSort}
	wig.AllCommands["CmdCheckHealth"] = wig.CmdDefinition{Desc: "Check health of dependencies", Fn: CmdCheckHealth}
	wig.AllCommands["checkhealth"] = wig.CmdDefinition{Desc: "Check health of dependencies", Fn: CmdCheckHealth}
	wig.AllCommands["CmdOpenConfig"] = wig.CmdDefinition{Desc: "Open config file", Fn: ui.ConfigPopupInit}
	wig.AllCommands["config"] = wig.CmdDefinition{Desc: "Open config file", Fn: ui.ConfigPopupInit}
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
	wig.AllCommands["w"] = wig.CmdDefinition{Desc: "Save", Fn: CmdSaveFileWithFeedback}
	wig.AllCommands["wq"] = wig.CmdDefinition{Desc: "Save and quit", Fn: func(ctx wig.Context) {
		if ctx.Buf != nil && (ctx.Buf.FilePath == "" || strings.HasPrefix(ctx.Buf.FilePath, "[")) && ctx.Char == "" {
			ctx.Editor.EchoMessage("No file name")
			return
		}
		CmdSaveFileWithFeedback(ctx)
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
	wig.AllCommands["CmdWindowHSplit"] = wig.CmdDefinition{Desc: "Horizontal split", Fn: CmdWindowHSplitLimited}
	wig.AllCommands["vs"] = wig.CmdDefinition{Desc: "Vertical split", Fn: CmdWindowVSplitLimited}
	wig.AllCommands["sp"] = wig.CmdDefinition{Desc: "Horizontal split", Fn: CmdWindowHSplitLimited}
	wig.AllCommands["on"] = wig.CmdDefinition{Desc: "Close other windows", Fn: wig.CmdWindowCloseOther}
	wig.AllCommands["only"] = wig.CmdDefinition{Desc: "Close other windows", Fn: wig.CmdWindowCloseOther}
	wig.AllCommands["close"] = wig.CmdDefinition{Desc: "Close window", Fn: wig.CmdWindowClose}

	// Additional commands used in default keymap
	wig.AllCommands["CmdExecute"] = wig.CmdDefinition{Desc: "Execute buffer", Fn: CmdExecute}
	wig.AllCommands["CmdCurrentBufferDirFilePicker"] = wig.CmdDefinition{Desc: "Find file in current dir", Fn: CmdCurrentBufferDirFilePicker}
	wig.AllCommands["CmdGotoDefinition"] = wig.CmdDefinition{Desc: "Go to definition", Fn: CmdGotoDefinition}
	wig.AllCommands["CmdTagJump"] = wig.CmdDefinition{Desc: "Jump to tag under cursor", Fn: CmdTagJump}
	wig.AllCommands["tag"] = wig.CmdDefinition{Desc: "Jump to tag", Fn: CmdTag}
	wig.AllCommands["CmdLspStatus"] = wig.CmdDefinition{Desc: "Show LSP connection status", Fn: CmdLspStatus}
	wig.AllCommands["lspstatus"] = wig.CmdDefinition{Desc: "Show LSP connection status", Fn: CmdLspStatus}
	wig.AllCommands["CmdCtagdSaved"] = wig.CmdDefinition{Desc: "Notify ctagd of saved file", Fn: CmdCtagdSaved}
	wig.AllCommands["CmdCtagdGotoDefinition"] = wig.CmdDefinition{Desc: "Go to definition (ctagd)", Fn: CmdCtagdGotoDefinition}
	wig.AllCommands["CmdCtagdGoto"] = wig.CmdDefinition{Desc: "Jump to symbol (ctagd)", Fn: CmdCtagdGoto}
	wig.AllCommands["CmdCtagdWorkspaceSymbols"] = wig.CmdDefinition{Desc: "Search workspace symbols (ctagd)", Fn: CmdCtagdWorkspaceSymbols}
	wig.AllCommands["CmdCtagdStatus"] = wig.CmdDefinition{Desc: "Show ctagd daemon status", Fn: CmdCtagdStatus}
	wig.AllCommands["ctagdstatus"] = wig.CmdDefinition{Desc: "Show ctagd daemon status", Fn: CmdCtagdStatus}
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
	wig.AllCommands["CmdWindowVSplit"] = wig.CmdDefinition{Desc: "Vertical split", Fn: CmdWindowVSplitLimited}
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
	wig.AllCommands["CmdRg"] = wig.CmdDefinition{Desc: "Ripgrep search", Fn: CmdRg}
	wig.AllCommands["rg"] = wig.CmdDefinition{Desc: "Ripgrep search", Fn: CmdRg}
	wig.AllCommands["CmdRgUnderCursor"] = wig.CmdDefinition{Desc: "Ripgrep search under cursor", Fn: CmdRgUnderCursor}
	wig.AllCommands["CmdOpenSavedSearch"] = wig.CmdDefinition{Desc: "Reopen rg search", Fn: CmdOpenSavedSearch}
	wig.AllCommands["copen"] = wig.CmdDefinition{Desc: "Open quickfix list", Fn: CmdQuickfixOpen}
	wig.AllCommands["CmdQuickfixOpen"] = wig.CmdDefinition{Desc: "Open quickfix list", Fn: CmdQuickfixOpen}
	wig.AllCommands["CmdLspDiagnosticsToQuickfix"] = wig.CmdDefinition{Desc: "Diagnostics to quickfix", Fn: CmdLspDiagnosticsToQuickfix}
	wig.AllCommands["diagnostics"] = wig.CmdDefinition{Desc: "Diagnostics to quickfix", Fn: CmdLspDiagnosticsToQuickfix}
	wig.AllCommands["diags"] = wig.CmdDefinition{Desc: "Diagnostics to quickfix", Fn: CmdLspDiagnosticsToQuickfix}
	wig.AllCommands["cclose"] = wig.CmdDefinition{Desc: "Close quickfix window", Fn: func(ctx wig.Context) {
		if len(ctx.Editor.Windows()) > 1 {
			wig.CmdWindowClose(ctx)
		} else {
			wig.CmdBufferCycle(ctx)
		}
	}}
	wig.AllCommands["CmdWorkspaceSwitch_1"] = wig.CmdDefinition{Desc: "Switch to workspace 1", Fn: wig.CmdWorkspaceSwitch_1}
	wig.AllCommands["CmdWorkspaceSwitch_2"] = wig.CmdDefinition{Desc: "Switch to workspace 2", Fn: wig.CmdWorkspaceSwitch_2}
	wig.AllCommands["CmdWorkspaceSwitch_3"] = wig.CmdDefinition{Desc: "Switch to workspace 3", Fn: wig.CmdWorkspaceSwitch_3}
	wig.AllCommands["CmdWorkspaceSwitch_4"] = wig.CmdDefinition{Desc: "Switch to workspace 4", Fn: wig.CmdWorkspaceSwitch_4}
	wig.AllCommands["CmdWorkspaceSwitch_5"] = wig.CmdDefinition{Desc: "Switch to workspace 5", Fn: wig.CmdWorkspaceSwitch_5}
	wig.AllCommands["CmdWorkspaceSwitch_6"] = wig.CmdDefinition{Desc: "Switch to workspace 6", Fn: wig.CmdWorkspaceSwitch_6}
	wig.AllCommands["CmdWorkspaceSwitch_7"] = wig.CmdDefinition{Desc: "Switch to workspace 7", Fn: wig.CmdWorkspaceSwitch_7}
	wig.AllCommands["CmdWorkspaceSwitch_8"] = wig.CmdDefinition{Desc: "Switch to workspace 8", Fn: wig.CmdWorkspaceSwitch_8}
	wig.AllCommands["CmdWorkspaceSwitch_9"] = wig.CmdDefinition{Desc: "Switch to workspace 9", Fn: wig.CmdWorkspaceSwitch_9}
	wig.AllCommands["CmdWorkspaceSwitch_0"] = wig.CmdDefinition{Desc: "Switch to workspace 0", Fn: wig.CmdWorkspaceSwitch_0}

	wig.AllCommands["CmdWindowMoveToWorkspace_1"] = wig.CmdDefinition{Desc: "Move window to workspace 1", Fn: wig.CmdWindowMoveToWorkspace_1}
	wig.AllCommands["CmdWindowMoveToWorkspace_2"] = wig.CmdDefinition{Desc: "Move window to workspace 2", Fn: wig.CmdWindowMoveToWorkspace_2}
	wig.AllCommands["CmdWindowMoveToWorkspace_3"] = wig.CmdDefinition{Desc: "Move window to workspace 3", Fn: wig.CmdWindowMoveToWorkspace_3}
	wig.AllCommands["CmdWindowMoveToWorkspace_4"] = wig.CmdDefinition{Desc: "Move window to workspace 4", Fn: wig.CmdWindowMoveToWorkspace_4}
	wig.AllCommands["CmdWindowMoveToWorkspace_5"] = wig.CmdDefinition{Desc: "Move window to workspace 5", Fn: wig.CmdWindowMoveToWorkspace_5}
	wig.AllCommands["CmdWindowMoveToWorkspace_6"] = wig.CmdDefinition{Desc: "Move window to workspace 6", Fn: wig.CmdWindowMoveToWorkspace_6}
	wig.AllCommands["CmdWindowMoveToWorkspace_7"] = wig.CmdDefinition{Desc: "Move window to workspace 7", Fn: wig.CmdWindowMoveToWorkspace_7}
	wig.AllCommands["CmdWindowMoveToWorkspace_8"] = wig.CmdDefinition{Desc: "Move window to workspace 8", Fn: wig.CmdWindowMoveToWorkspace_8}
	wig.AllCommands["CmdWindowMoveToWorkspace_9"] = wig.CmdDefinition{Desc: "Move window to workspace 9", Fn: wig.CmdWindowMoveToWorkspace_9}
	wig.AllCommands["CmdWindowMoveToWorkspace_0"] = wig.CmdDefinition{Desc: "Move window to workspace 0", Fn: wig.CmdWindowMoveToWorkspace_0}

	wig.AllCommands["wslist"] = wig.CmdDefinition{Desc: "List and switch workspaces", Fn: CmdWorkspaceListPicker}
	wig.AllCommands["wssave"] = wig.CmdDefinition{Desc: "Save workspaces", Fn: CmdWorkspaceSave}
	wig.AllCommands["CmdWorkspaceSave"] = wig.CmdDefinition{Desc: "Save workspaces", Fn: CmdWorkspaceSave}

	wig.AllCommands["CmdDeleteInsideFunction"] = wig.CmdDefinition{Desc: "Delete inside function", Fn: wig.CmdDeleteInsideFunction, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundFunction"] = wig.CmdDefinition{Desc: "Delete around function", Fn: wig.CmdDeleteAroundFunction, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideWord"] = wig.CmdDefinition{Desc: "Delete inside word", Fn: wig.CmdDeleteInsideWord, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundWord"] = wig.CmdDefinition{Desc: "Delete around word", Fn: wig.CmdDeleteAroundWord, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideParagraph"] = wig.CmdDefinition{Desc: "Delete inside paragraph", Fn: wig.CmdDeleteInsideParagraph, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundParagraph"] = wig.CmdDefinition{Desc: "Delete around paragraph", Fn: wig.CmdDeleteAroundParagraph, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideQuotesDouble"] = wig.CmdDefinition{Desc: "Delete inside double quotes", Fn: wig.CmdDeleteInsideQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundQuotesDouble"] = wig.CmdDefinition{Desc: "Delete around double quotes", Fn: wig.CmdDeleteAroundQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideQuotesSingle"] = wig.CmdDefinition{Desc: "Delete inside single quotes", Fn: wig.CmdDeleteInsideQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundQuotesSingle"] = wig.CmdDefinition{Desc: "Delete around single quotes", Fn: wig.CmdDeleteAroundQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideQuotesBacktick"] = wig.CmdDefinition{Desc: "Delete inside backtick quotes", Fn: wig.CmdDeleteInsideQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundQuotesBacktick"] = wig.CmdDefinition{Desc: "Delete around backtick quotes", Fn: wig.CmdDeleteAroundQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideParen"] = wig.CmdDefinition{Desc: "Delete inside parentheses", Fn: wig.CmdDeleteInsideParen, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundParen"] = wig.CmdDefinition{Desc: "Delete around parentheses", Fn: wig.CmdDeleteAroundParen, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideBrace"] = wig.CmdDefinition{Desc: "Delete inside braces", Fn: wig.CmdDeleteInsideBrace, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundBrace"] = wig.CmdDefinition{Desc: "Delete around braces", Fn: wig.CmdDeleteAroundBrace, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideBracket"] = wig.CmdDefinition{Desc: "Delete inside brackets", Fn: wig.CmdDeleteInsideBracket, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundBracket"] = wig.CmdDefinition{Desc: "Delete around brackets", Fn: wig.CmdDeleteAroundBracket, Repeatable: true}
	wig.AllCommands["CmdDeleteInsideAngle"] = wig.CmdDefinition{Desc: "Delete inside angle brackets", Fn: wig.CmdDeleteInsideAngle, Repeatable: true}
	wig.AllCommands["CmdDeleteAroundAngle"] = wig.CmdDefinition{Desc: "Delete around angle brackets", Fn: wig.CmdDeleteAroundAngle, Repeatable: true}

	wig.AllCommands["CmdChangeInsideFunction"] = wig.CmdDefinition{Desc: "Change inside function", Fn: wig.CmdChangeInsideFunction, Repeatable: true}
	wig.AllCommands["CmdChangeAroundFunction"] = wig.CmdDefinition{Desc: "Change around function", Fn: wig.CmdChangeAroundFunction, Repeatable: true}
	wig.AllCommands["CmdChangeInsideWord"] = wig.CmdDefinition{Desc: "Change inside word", Fn: wig.CmdChangeInsideWord, Repeatable: true}
	wig.AllCommands["CmdChangeAroundWord"] = wig.CmdDefinition{Desc: "Change around word", Fn: wig.CmdChangeAroundWord, Repeatable: true}
	wig.AllCommands["CmdChangeInsideParagraph"] = wig.CmdDefinition{Desc: "Change inside paragraph", Fn: wig.CmdChangeInsideParagraph, Repeatable: true}
	wig.AllCommands["CmdChangeAroundParagraph"] = wig.CmdDefinition{Desc: "Change around paragraph", Fn: wig.CmdChangeAroundParagraph, Repeatable: true}
	wig.AllCommands["CmdChangeInsideQuotesDouble"] = wig.CmdDefinition{Desc: "Change inside double quotes", Fn: wig.CmdChangeInsideQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdChangeAroundQuotesDouble"] = wig.CmdDefinition{Desc: "Change around double quotes", Fn: wig.CmdChangeAroundQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdChangeInsideQuotesSingle"] = wig.CmdDefinition{Desc: "Change inside single quotes", Fn: wig.CmdChangeInsideQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdChangeAroundQuotesSingle"] = wig.CmdDefinition{Desc: "Change around single quotes", Fn: wig.CmdChangeAroundQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdChangeInsideQuotesBacktick"] = wig.CmdDefinition{Desc: "Change inside backtick quotes", Fn: wig.CmdChangeInsideQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdChangeAroundQuotesBacktick"] = wig.CmdDefinition{Desc: "Change around backtick quotes", Fn: wig.CmdChangeAroundQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdChangeInsideParen"] = wig.CmdDefinition{Desc: "Change inside parentheses", Fn: wig.CmdChangeInsideParen, Repeatable: true}
	wig.AllCommands["CmdChangeAroundParen"] = wig.CmdDefinition{Desc: "Change around parentheses", Fn: wig.CmdChangeAroundParen, Repeatable: true}
	wig.AllCommands["CmdChangeInsideBrace"] = wig.CmdDefinition{Desc: "Change inside braces", Fn: wig.CmdChangeInsideBrace, Repeatable: true}
	wig.AllCommands["CmdChangeAroundBrace"] = wig.CmdDefinition{Desc: "Change around braces", Fn: wig.CmdChangeAroundBrace, Repeatable: true}
	wig.AllCommands["CmdChangeInsideBracket"] = wig.CmdDefinition{Desc: "Change inside brackets", Fn: wig.CmdChangeInsideBracket, Repeatable: true}
	wig.AllCommands["CmdChangeAroundBracket"] = wig.CmdDefinition{Desc: "Change around brackets", Fn: wig.CmdChangeAroundBracket, Repeatable: true}
	wig.AllCommands["CmdChangeInsideAngle"] = wig.CmdDefinition{Desc: "Change inside angle brackets", Fn: wig.CmdChangeInsideAngle, Repeatable: true}
	wig.AllCommands["CmdChangeAroundAngle"] = wig.CmdDefinition{Desc: "Change around angle brackets", Fn: wig.CmdChangeAroundAngle, Repeatable: true}

	wig.AllCommands["CmdYankInsideFunction"] = wig.CmdDefinition{Desc: "Yank inside function", Fn: wig.CmdYankInsideFunction}
	wig.AllCommands["CmdYankAroundFunction"] = wig.CmdDefinition{Desc: "Yank around function", Fn: wig.CmdYankAroundFunction}
	wig.AllCommands["CmdYankInsideWord"] = wig.CmdDefinition{Desc: "Yank inside word", Fn: wig.CmdYankInsideWord}
	wig.AllCommands["CmdYankAroundWord"] = wig.CmdDefinition{Desc: "Yank around word", Fn: wig.CmdYankAroundWord}
	wig.AllCommands["CmdYankInsideParagraph"] = wig.CmdDefinition{Desc: "Yank inside paragraph", Fn: wig.CmdYankInsideParagraph}
	wig.AllCommands["CmdYankAroundParagraph"] = wig.CmdDefinition{Desc: "Yank around paragraph", Fn: wig.CmdYankAroundParagraph}
	wig.AllCommands["CmdYankInsideQuotesDouble"] = wig.CmdDefinition{Desc: "Yank inside double quotes", Fn: wig.CmdYankInsideQuotesDouble}
	wig.AllCommands["CmdYankAroundQuotesDouble"] = wig.CmdDefinition{Desc: "Yank around double quotes", Fn: wig.CmdYankAroundQuotesDouble}
	wig.AllCommands["CmdYankInsideQuotesSingle"] = wig.CmdDefinition{Desc: "Yank inside single quotes", Fn: wig.CmdYankInsideQuotesSingle}
	wig.AllCommands["CmdYankAroundQuotesSingle"] = wig.CmdDefinition{Desc: "Yank around single quotes", Fn: wig.CmdYankAroundQuotesSingle}
	wig.AllCommands["CmdYankInsideQuotesBacktick"] = wig.CmdDefinition{Desc: "Yank inside backtick quotes", Fn: wig.CmdYankInsideQuotesBacktick}
	wig.AllCommands["CmdYankAroundQuotesBacktick"] = wig.CmdDefinition{Desc: "Yank around backtick quotes", Fn: wig.CmdYankAroundQuotesBacktick}
	wig.AllCommands["CmdYankInsideParen"] = wig.CmdDefinition{Desc: "Yank inside parentheses", Fn: wig.CmdYankInsideParen}
	wig.AllCommands["CmdYankAroundParen"] = wig.CmdDefinition{Desc: "Yank around parentheses", Fn: wig.CmdYankAroundParen}
	wig.AllCommands["CmdYankInsideBrace"] = wig.CmdDefinition{Desc: "Yank inside braces", Fn: wig.CmdYankInsideBrace}
	wig.AllCommands["CmdYankAroundBrace"] = wig.CmdDefinition{Desc: "Yank around braces", Fn: wig.CmdYankAroundBrace}
	wig.AllCommands["CmdYankInsideBracket"] = wig.CmdDefinition{Desc: "Yank inside brackets", Fn: wig.CmdYankInsideBracket}
	wig.AllCommands["CmdYankAroundBracket"] = wig.CmdDefinition{Desc: "Yank around brackets", Fn: wig.CmdYankAroundBracket}
	wig.AllCommands["CmdYankInsideAngle"] = wig.CmdDefinition{Desc: "Yank inside angle brackets", Fn: wig.CmdYankInsideAngle}
	wig.AllCommands["CmdYankAroundAngle"] = wig.CmdDefinition{Desc: "Yank around angle brackets", Fn: wig.CmdYankAroundAngle}

	wig.AllCommands["CmdCommentInsideFunction"] = wig.CmdDefinition{Desc: "Comment inside function", Fn: wig.CmdCommentInsideFunction, Repeatable: true}
	wig.AllCommands["CmdCommentAroundFunction"] = wig.CmdDefinition{Desc: "Comment around function", Fn: wig.CmdCommentAroundFunction, Repeatable: true}
	wig.AllCommands["CmdCommentInsideWord"] = wig.CmdDefinition{Desc: "Comment inside word", Fn: wig.CmdCommentInsideWord, Repeatable: true}
	wig.AllCommands["CmdCommentAroundWord"] = wig.CmdDefinition{Desc: "Comment around word", Fn: wig.CmdCommentAroundWord, Repeatable: true}
	wig.AllCommands["CmdCommentInsideParagraph"] = wig.CmdDefinition{Desc: "Comment inside paragraph", Fn: wig.CmdCommentInsideParagraph, Repeatable: true}
	wig.AllCommands["CmdCommentAroundParagraphText"] = wig.CmdDefinition{Desc: "Comment around paragraph", Fn: wig.CmdCommentAroundParagraphText, Repeatable: true}
	wig.AllCommands["CmdCommentInsideQuotesDouble"] = wig.CmdDefinition{Desc: "Comment inside double quotes", Fn: wig.CmdCommentInsideQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdCommentAroundQuotesDouble"] = wig.CmdDefinition{Desc: "Comment around double quotes", Fn: wig.CmdCommentAroundQuotesDouble, Repeatable: true}
	wig.AllCommands["CmdCommentInsideQuotesSingle"] = wig.CmdDefinition{Desc: "Comment inside single quotes", Fn: wig.CmdCommentInsideQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdCommentAroundQuotesSingle"] = wig.CmdDefinition{Desc: "Comment around single quotes", Fn: wig.CmdCommentAroundQuotesSingle, Repeatable: true}
	wig.AllCommands["CmdCommentInsideQuotesBacktick"] = wig.CmdDefinition{Desc: "Comment inside backtick quotes", Fn: wig.CmdCommentInsideQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdCommentAroundQuotesBacktick"] = wig.CmdDefinition{Desc: "Comment around backtick quotes", Fn: wig.CmdCommentAroundQuotesBacktick, Repeatable: true}
	wig.AllCommands["CmdCommentInsideParen"] = wig.CmdDefinition{Desc: "Comment inside parentheses", Fn: wig.CmdCommentInsideParen, Repeatable: true}
	wig.AllCommands["CmdCommentAroundParen"] = wig.CmdDefinition{Desc: "Comment around parentheses", Fn: wig.CmdCommentAroundParen, Repeatable: true}
	wig.AllCommands["CmdCommentInsideBrace"] = wig.CmdDefinition{Desc: "Comment inside braces", Fn: wig.CmdCommentInsideBrace, Repeatable: true}
	wig.AllCommands["CmdCommentAroundBrace"] = wig.CmdDefinition{Desc: "Comment around braces", Fn: wig.CmdCommentAroundBrace, Repeatable: true}
	wig.AllCommands["CmdCommentInsideBracket"] = wig.CmdDefinition{Desc: "Comment inside brackets", Fn: wig.CmdCommentInsideBracket, Repeatable: true}
	wig.AllCommands["CmdCommentAroundBracket"] = wig.CmdDefinition{Desc: "Comment around brackets", Fn: wig.CmdCommentAroundBracket, Repeatable: true}
	wig.AllCommands["CmdCommentInsideAngle"] = wig.CmdDefinition{Desc: "Comment inside angle brackets", Fn: wig.CmdCommentInsideAngle, Repeatable: true}
	wig.AllCommands["CmdCommentAroundAngle"] = wig.CmdDefinition{Desc: "Comment around angle brackets", Fn: wig.CmdCommentAroundAngle, Repeatable: true}

	wig.AllCommands["CmdDummyNA"] = wig.CmdDefinition{Desc: "Disable key", Fn: wig.CmdDummyNA}
	wig.AllCommands["nop"] = wig.CmdDefinition{Desc: "NOP", Fn: wig.CmdDummyNA}
}
