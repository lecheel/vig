package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/autocomplete"
	"github.com/firstrow/wig/commands"
	"github.com/firstrow/wig/ui"
	"github.com/pelletier/go-toml/v2"
)

type UserConfig struct {
	Editor EditorSettings `toml:"editor"`
	Keys   UserKeysConfig `toml:"keys"`
}

type EditorSettings struct {
	Leader              *string `toml:"leader"`
	CommentStyle        *string `toml:"comment_style"`
	GcMotion            *bool   `toml:"gc_motion"`
	Theme               *string `toml:"theme"`
	ShowLineNumbers     *bool   `toml:"show_line_numbers"`
	RelativeLineNumbers *bool   `toml:"relative_line_numbers"`
	CurrentLineAbsolute *bool   `toml:"current_line_absolute"`
	FormatOnSave        *bool   `toml:"format_on_save"`
	GitStatusView       *string `toml:"git_status_view"`
	GitBlameView        *string `toml:"git_blame_view"`
	IndentGuides        *bool   `toml:"indent_guides"`
	LspEnabled          *bool   `toml:"lsp_enabled"`
	WhichKeyFormat      *string `toml:"which_key_format"`
}

type UserKeysConfig struct {
	Normal      map[string]string `toml:"normal"`
	Insert      map[string]string `toml:"insert"`
	Visual      map[string]string `toml:"visual"`
	VisualLine  map[string]string `toml:"visual_line"`
	VisualBlock map[string]string `toml:"visual_block"`
}

func normalizeLeader(leader string) string {
	switch strings.ToLower(strings.TrimSpace(leader)) {
	case "", "space", "<space>", " ":
		return "Space"
	default:
		return strings.TrimSpace(leader)
	}
}

// LoadUserConfig reads ~/.config/wig/config.toml for editor settings and keymaps
func LoadUserConfig() (wig.EditorConfig, wig.ModeKeyMap) {
	editorCfg := wig.EditorConfig{
		Leader:              "Space",
		CommentStyle:        "standard",
		Theme:               "naysayer",
		ShowLineNumbers:     true,
		RelativeLineNumbers: true,
		CurrentLineAbsolute: true,
		FormatOnSave:        false,
		GitStatusView:       "full",
		GitBlameView:        "split",
		IndentGuides:        false,
		LspEnabled:          true,
		WhichKeyFormat:      "words",
	}
	userMap := wig.ModeKeyMap{
		wig.MODE_NORMAL:       wig.KeyMap{},
		wig.MODE_INSERT:       wig.KeyMap{},
		wig.MODE_VISUAL:       wig.KeyMap{},
		wig.MODE_VISUAL_LINE:  wig.KeyMap{},
		wig.MODE_VISUAL_BLOCK: wig.KeyMap{},
	}

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "wig", "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return editorCfg, userMap // File doesn't exist, return defaults
	}

	var cfg UserConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return editorCfg, userMap
	}

	// Apply editor settings if they were provided in the TOML
	if cfg.Editor.Leader != nil {
		editorCfg.Leader = normalizeLeader(*cfg.Editor.Leader)
	}
	if cfg.Editor.CommentStyle != nil {
		editorCfg.CommentStyle = *cfg.Editor.CommentStyle
	}
	if cfg.Editor.GcMotion != nil {
		if *cfg.Editor.GcMotion {
			editorCfg.CommentStyle = "standard"
		} else {
			editorCfg.CommentStyle = "simple"
		}
	}
	if cfg.Editor.Theme != nil {
		editorCfg.Theme = *cfg.Editor.Theme
	}
	if cfg.Editor.ShowLineNumbers != nil {
		editorCfg.ShowLineNumbers = *cfg.Editor.ShowLineNumbers
	}
	if cfg.Editor.RelativeLineNumbers != nil {
		editorCfg.RelativeLineNumbers = *cfg.Editor.RelativeLineNumbers
	}
	if cfg.Editor.CurrentLineAbsolute != nil {
		editorCfg.CurrentLineAbsolute = *cfg.Editor.CurrentLineAbsolute
	}
	if cfg.Editor.FormatOnSave != nil {
		editorCfg.FormatOnSave = *cfg.Editor.FormatOnSave
	}
	if cfg.Editor.GitStatusView != nil {
		editorCfg.GitStatusView = *cfg.Editor.GitStatusView
	}
	if cfg.Editor.GitBlameView != nil {
		editorCfg.GitBlameView = *cfg.Editor.GitBlameView
	}
	if cfg.Editor.IndentGuides != nil {
		editorCfg.IndentGuides = *cfg.Editor.IndentGuides
	}
	if cfg.Editor.LspEnabled != nil {
		editorCfg.LspEnabled = *cfg.Editor.LspEnabled
	}
	if cfg.Editor.WhichKeyFormat != nil {
		editorCfg.WhichKeyFormat = *cfg.Editor.WhichKeyFormat
	}
	resolve := func(name string) any {
		if def, ok := wig.AllCommands[name]; ok {
			return def.Fn
		}
		return nil
	}

	expandKey := func(k string) string {
		k = strings.ReplaceAll(k, "<leader>", editorCfg.Leader)
		k = strings.ReplaceAll(k, "<Leader>", editorCfg.Leader)
		if k == "<space>" || k == "<Space>" || k == " " {
			return "Space"
		}
		return k
	}

	for key, cmdName := range cfg.Keys.Normal {
		if fn := resolve(cmdName); fn != nil {
			userMap[wig.MODE_NORMAL][expandKey(key)] = fn
		}
	}
	for key, cmdName := range cfg.Keys.Insert {
		if fn := resolve(cmdName); fn != nil {
			userMap[wig.MODE_INSERT][expandKey(key)] = fn
		}
	}
	for key, cmdName := range cfg.Keys.Visual {
		if fn := resolve(cmdName); fn != nil {
			userMap[wig.MODE_VISUAL][expandKey(key)] = fn
		}
	}
	for key, cmdName := range cfg.Keys.VisualLine {
		if fn := resolve(cmdName); fn != nil {
			userMap[wig.MODE_VISUAL_LINE][expandKey(key)] = fn
		}
	}
	for key, cmdName := range cfg.Keys.VisualBlock {
		if fn := resolve(cmdName); fn != nil {
			userMap[wig.MODE_VISUAL_BLOCK][expandKey(key)] = fn
		}
	}

	return editorCfg, userMap
}

func DefaultKeyMap(args ...string) wig.ModeKeyMap {
	leader := "Space"
	commentStyle := "standard"

	if len(args) > 0 && args[0] != "" {
		leader = normalizeLeader(args[0])
	}
	if len(args) > 1 && args[1] != "" {
		commentStyle = strings.ToLower(args[1])
	}

	var gcMapping any
	if commentStyle == "simple" {
		gcMapping = wig.CmdToggleComment
	} else {
		gcMapping = wig.KeyMap{
			"c": wig.CmdCommentLine,
			"j": wig.CmdCommentLineDown,
			"k": wig.CmdCommentLineUp,
			"$": wig.CmdCommentEndOfLine,
			"w": wig.CmdCommentWord,
			"G": wig.CmdCommentEndOfFile,
			"g": wig.CmdCommentStartOfFile,
			"i": wig.CmdCommentInside,
			"a": wig.CmdCommentAround,
		}
	}

	return wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"F1":     commands.CmdGitView,
			"F2":     commands.CmdFormatBufferAndSave,
			"F3":     commands.CmdMakeTest,
			"F5":     commands.CmdMakeBuild,
			"F8":     commands.CmdFunctionList,
			"F11":    commands.CmdOpenSavedSearch,
			"F12":    ui.CmdMiniHelp,
			"ctrl+b": commands.CmdMRUBufferPicker,
			"ctrl+e": wig.CmdScrollDownLine,
			"ctrl+y": wig.CmdScrollUpLine,
			"h":      wig.CmdCursorLeft,
			"l":      wig.CmdCursorRight,
			"j":      wig.CmdCursorLineDown,
			"k":      wig.CmdCursorLineUp,
			"Left":   wig.CmdCursorLeft,
			"Right":  wig.CmdCursorRight,
			"Up":     wig.CmdCursorLineUp,
			"Down":   wig.CmdCursorLineDown,
			"Home":   wig.CmdCursorBeginningOfTheLine,
			"End":    wig.CmdGotoLineEnd,
			"PgUp":   wig.CmdScrollUpPage,
			"PgDn":   wig.CmdScrollDownPage,
			"i":      wig.CmdEnterInsertMode,
			"v":      wig.CmdVisualMode,
			"V":      wig.CmdVisualLineMode,
			"ctrl+v": wig.CmdVisualBlockMode,
			"a":      wig.CmdEnterInsertModeAppend,
			"A":      wig.CmdAppendLine,
			"w":      wig.CmdForwardWord,
			"b":      wig.CmdBackwardWord,
			"x":      wig.CmdDeleteCharForward,
			"X":      wig.CmdDeleteCharBackward,
			"Delete": wig.CmdDeleteCharForward,
			"Insert": commands.CmdToggleBool,
			"^":      wig.CmdCursorFirstNonBlank,
			"$":      wig.CmdGotoLineEnd,
			"0":      wig.CmdCursorBeginningOfTheLine,
			"o":      wig.CmdLineOpenBelow,
			"O":      wig.CmdLineOpenAbove,
			"J":      wig.CmdJoinNextLine,
			"K":      commands.CmdRgUnderCursor,
			"p":      wig.CmdYankPut,
			"P":      wig.CmdYankPutBefore,
			"s":      ui.CmdEasyMotion,
			"r":      wig.CmdReplaceChar,
			"f":      wig.CmdForwardToChar,
			"t":      wig.CmdForwardBeforeChar,
			"F":      wig.CmdBackwardChar,
			"G":      wig.CmdGotoLineEndOfFile,
			"n":      wig.CmdSearchNext,
			"N":      wig.CmdSearchPrev,
			"%":      wig.CmdMatchPair,
			"u":      wig.CmdUndo,
			"ctrl+r": wig.CmdRedo,
			":":      ui.CmdLineInit,
			"/":      ui.CmdSearchPromptInit,
			";":      commands.CmdBufferPicker,
			"*":      commands.CmdSearchWordUnderCursor,
			"q":      wig.CmdMacroRecord,
			"@":      wig.CmdMacroPlay,
			".":      wig.CmdMacroRepeat,
			"m":      wig.CmdSetMark,
			"`":      wig.CmdGotoMark,
			"\"":     wig.CmdSelectRegister,
			"c": wig.KeyMap{
				"$": wig.CmdChangeEndOfLine,
				"c": wig.CmdChangeLine,
				"w": wig.CmdChangeWord,
				"a": wig.CmdChangeAround,
				"i": wig.CmdChangeInside,
				"f": wig.CmdChangeTo,
				"t": wig.CmdChangeBefore,
			},
			"d": wig.KeyMap{
				"d": wig.CmdDeleteLine,
				"w": wig.CmdDeleteWord,
				"i": wig.CmdDeleteInside,
				"a": wig.CmdDeleteAround,
				"f": wig.CmdDeleteTo,
				"t": wig.CmdDeleteBefore,
				"$": wig.CmdDeleteEndOfLine,
			},
			"y": wig.KeyMap{
				"y": wig.CmdYank,
				"$": wig.CmdYankEol,
				"t": wig.CmdYankBeforeChar,
				"f": wig.CmdYankToChar,
				"i": wig.CmdYankInside,
				"a": wig.CmdYankAround,
			},
			"g": wig.KeyMap{
				"g": wig.CmdGotoLine0,
				"f": wig.CmdGotoFile,
				"F": wig.CmdGotoFileOtherWindow,
				"d": commands.CmdGotoDefinition,
				"O": commands.CmdGotoDefinitionOtherWindow,
				"o": commands.CmdViewDefinitionOtherWindow,
				"c": gcMapping,
			},
			"ctrl+c": wig.KeyMap{
				"ctrl+c": commands.CmdExecute,
				"ctrl+x": commands.CmdExit,
			},
			"ctrl+w": wig.KeyMap{
				"v":      wig.CmdWindowVSplit,
				"w":      wig.CmdWindowNext,
				"q":      wig.CmdWindowClose,
				"o":      wig.CmdWindowCloseOther,
				"c":      wig.CmdWindowCloseAndKillBuffer,
				"ctrl+w": wig.CmdWindowNext,
				"t":      wig.CmdWindowToggleLayout,
			},
			"]": wig.KeyMap{
				"]": wig.CmdJumpForward,
				"h": commands.CmdGitHunkNext,
			},
			"[": wig.KeyMap{
				"[": wig.CmdJumpBack,
				"h": commands.CmdGitHunkPrev,
			},
			leader: wig.KeyMap{
				"/": commands.CmdSearchProject,
				"?": commands.CmdCommandPalettePicker,
				"`": wig.CmdBufferCycle,
				"*": commands.CmdProjectSearchWordUnderCursor,
				"h": commands.CmdLspHover,
				"e": commands.CmdLspShowDiagnostics,
				"b": wig.KeyMap{
					"b": commands.CmdBufferPicker,
					"k": wig.CmdKillBuffer,
				},
				"f": commands.CmdFindProjectFilePicker,
				"F": commands.CmdCurrentBufferDirFilePicker,
				"s": wig.KeyMap{
					"s": commands.CmdSearchLine,
					"n": wig.CmdVisitNextLine,
					"p": wig.CmdVisitPrevLine,
				},
				"t": commands.CmdThemeSelect,
				"i": wig.CmdToggleIndentGuides,
				"y": commands.CmdClipboardCopy,
				"p": wig.KeyMap{
					"v": commands.CmdClipboardPaste,
					"p": commands.CmdClipboardPasteAll,
				},
				"g": wig.KeyMap{
					"r": commands.CmdGitHunkRevert,
					"p": commands.CmdGitHunkPreview,
					"d": commands.CmdGitBlameCommit,
					"b": commands.CmdGitBlame,
					"g": commands.CmdGitView,
					"f": commands.CmdGitFilesPicker,
				},
			},
		},
		wig.MODE_VISUAL: wig.KeyMap{
			"Esc":    wig.CmdNormalMode,
			":":      ui.CmdLineInit,
			"ctrl+e": wig.WithSelection(wig.CmdScrollDownLine),
			"ctrl+y": wig.WithSelection(wig.CmdScrollUpLine),
			"w":      wig.WithSelection(wig.CmdForwardWord),
			"b":      wig.WithSelection(wig.CmdBackwardWord),
			"h":      wig.WithSelection(wig.CmdCursorLeft),
			"l":      wig.WithSelection(wig.CmdCursorRight),
			"j":      wig.WithSelection(wig.CmdCursorLineDown),
			"k":      wig.WithSelection(wig.CmdCursorLineUp),
			"Left":   wig.WithSelection(wig.CmdCursorLeft),
			"Right":  wig.WithSelection(wig.CmdCursorRight),
			"Up":     wig.WithSelection(wig.CmdCursorLineUp),
			"Down":   wig.WithSelection(wig.CmdCursorLineDown),
			"Home":   wig.WithSelection(wig.CmdCursorBeginningOfTheLine),
			"End":    wig.WithSelection(wig.CmdGotoLineEnd),
			"PgUp":   wig.WithSelection(wig.CmdScrollUpPage),
			"PgDn":   wig.WithSelection(wig.CmdScrollDownPage),
			"$":      wig.WithSelection(wig.CmdGotoLineEnd),
			"0":      wig.WithSelection(wig.CmdCursorBeginningOfTheLine),
			"G":      wig.WithSelection(wig.CmdGotoLineEndOfFile),
			"s":      ui.CmdEasyMotion,
			"f":      wig.CmdForwardToChar,
			"t":      wig.CmdForwardBeforeChar,
			"x":      wig.CmdSelectionDelete,
			"d":      wig.CmdSelectionDelete,
			"y":      wig.CmdYank,
			"p":      wig.CmdYankPut,
			"c":      wig.CmdSelectionChange,
			"*":      commands.CmdSearchWordUnderCursor,
			"%":      wig.WithSelection(wig.CmdMatchPair),
			"g": wig.KeyMap{
				"g": wig.WithSelection(wig.CmdGotoLine0),
				"c": wig.CmdToggleComment,
			},
			leader: wig.KeyMap{
				"y": commands.CmdClipboardCopy,
				"p": wig.KeyMap{
					"v": commands.CmdClipboardPaste,
					"p": commands.CmdClipboardPasteAll,
				},
			},
		},
		wig.MODE_VISUAL_LINE: wig.KeyMap{
			"Esc":    wig.CmdNormalMode,
			":":      ui.CmdLineInit,
			"ctrl+e": wig.CmdScrollDownLine,
			"ctrl+y": wig.CmdScrollUpLine,
			"j":      wig.WithSelection(wig.CmdCursorLineDown),
			"k":      wig.WithSelection(wig.CmdCursorLineUp),
			"h":      wig.CmdCursorLeft,
			"l":      wig.CmdCursorRight,
			"Left":   wig.CmdCursorLeft,
			"Right":  wig.CmdCursorRight,
			"Up":     wig.WithSelection(wig.CmdCursorLineUp),
			"Down":   wig.WithSelection(wig.CmdCursorLineDown),
			"Home":   wig.CmdCursorBeginningOfTheLine,
			"End":    wig.CmdGotoLineEnd,
			"PgUp":   wig.CmdScrollUpPage,
			"PgDn":   wig.CmdScrollDownPage,
			"G":      wig.WithSelection(wig.CmdGotoLineEndOfFile),
			"x":      wig.CmdSelectionDelete,
			"d":      wig.CmdSelectionDelete,
			"y":      wig.CmdYank,
			"p":      wig.CmdYankPut,
			"%":      wig.WithSelection(wig.CmdMatchPair),
			"g": wig.KeyMap{
				"g": wig.WithSelection(wig.CmdGotoLine0),
				"c": wig.CmdToggleComment,
			},
			leader: wig.KeyMap{
				"y": commands.CmdClipboardCopy,
				"p": wig.KeyMap{
					"v": commands.CmdClipboardPaste,
					"p": commands.CmdClipboardPasteAll,
				},
			},
		},
		wig.MODE_VISUAL_BLOCK: wig.KeyMap{
			"Esc":    wig.CmdNormalMode,
			":":      ui.CmdLineInit,
			"ctrl+e": wig.WithSelection(wig.CmdScrollDownLine),
			"ctrl+y": wig.WithSelection(wig.CmdScrollUpLine),
			"j":      wig.WithSelection(wig.CmdCursorLineDown),
			"k":      wig.WithSelection(wig.CmdCursorLineUp),
			"h":      wig.WithSelection(wig.CmdCursorLeft),
			"l":      wig.WithSelection(wig.CmdCursorRight),
			"Left":   wig.WithSelection(wig.CmdCursorLeft),
			"Right":  wig.WithSelection(wig.CmdCursorRight),
			"Up":     wig.WithSelection(wig.CmdCursorLineUp),
			"Down":   wig.WithSelection(wig.CmdCursorLineDown),
			"Home":   wig.WithSelection(wig.CmdCursorBeginningOfTheLine),
			"End":    wig.WithSelection(wig.CmdGotoLineEnd),
			"PgUp":   wig.WithSelection(wig.CmdScrollUpPage),
			"PgDn":   wig.WithSelection(wig.CmdScrollDownPage),
			"$":      wig.WithSelection(wig.CmdGotoLineEnd),
			"0":      wig.WithSelection(wig.CmdCursorBeginningOfTheLine),
			"G":      wig.WithSelection(wig.CmdGotoLineEndOfFile),
			"I":      wig.CmdVisualBlockInsert,
			"d":      wig.CmdSelectionBlockDelete,
			"x":      wig.CmdSelectionBlockDelete,
			"y":      wig.CmdSelectionBlockYank,
		},
		wig.MODE_INSERT: wig.KeyMap{
			"Esc":    wig.CmdExitInsertMode,
			"ctrl+f": wig.CmdCursorRight,
			"ctrl+b": wig.CmdCursorLeft,
			"ctrl+j": wig.CmdCursorLineDown,
			"ctrl+k": wig.CmdCursorLineUp,
			"ctrl+n": wig.CmdAutocompleteTrigger,
			"ctrl+r": wig.CmdInsertRegister,
			"alt+/":  autocomplete.BufferComplete,
			"Left":   wig.CmdCursorLeft,
			"Right":  wig.CmdCursorRight,
			"Up":     wig.CmdCursorLineUp,
			"Down":   wig.CmdCursorLineDown,
			"Home":   wig.CmdCursorBeginningOfTheLine,
			"End":    wig.CmdGotoLineEnd,
			"PgUp":   wig.CmdScrollUpPage,
			"PgDn":   wig.CmdScrollDownPage,
			"Delete": wig.CmdDeleteCharForward,
		},
	}
}
