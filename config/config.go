package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/autocomplete"
	"github.com/firstrow/wig/commands"
	"github.com/firstrow/wig/ui"
	"github.com/pelletier/go-toml/v2"
)

var keyTokenizer = regexp.MustCompile(`(<[^>]+>|(?i:ctrl)\+\S+|(?i:alt)\+\S+|(?i:shift)\+\S+|(?i:meta)\+\S+|[A-Z][a-zA-Z0-9]*|.)`)

func tokenizeKey(k string) []string {
	return keyTokenizer.FindAllString(k, -1)
}

func assignSequence(currentMap wig.KeyMap, tokens []string, action any) {
	if len(tokens) == 0 {
		return
	}
	token := tokens[0]

	if len(tokens) == 1 {
		currentMap[token] = action
		return
	}

	var nextMap wig.KeyMap
	if existing, ok := currentMap[token]; ok {
		if m, ok := existing.(wig.KeyMap); ok {
			nextMap = m
		} else {
			nextMap = wig.KeyMap{}
		}
	} else {
		nextMap = wig.KeyMap{}
	}
	currentMap[token] = nextMap
	assignSequence(nextMap, tokens[1:], action)
}

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
	QuickfixView        *string `toml:"quickfix_view"`
	IndentGuides        *bool   `toml:"indent_guides"`
	LspEnabled          *bool   `toml:"lsp_enabled"`
	WhichKeyFormat      *string `toml:"which_key_format"`
	SameStatuslineColor *bool   `toml:"same_statusline_color"`
	StatuslineStyle     *string `toml:"statusline_style"`
	NotifyOnSave        *bool   `toml:"notify_on_save"`
	GitAiCommit         *bool   `toml:"git_ai_commit"`
	GitAiTool           *string `toml:"git_ai_tool"`
	SaveWorkspaces      *bool   `toml:"save_workspaces"`
	Workspaces          *bool   `toml:"workspaces"`
	Ws                  *bool   `toml:"ws"`
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
		QuickfixView:        "split",
		IndentGuides:        false,
		LspEnabled:          true,
		WhichKeyFormat:      "words",
		SameStatuslineColor: false,
		SaveWorkspaces:      false,
		StatuslineStyle:     "plain",
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
	if cfg.Editor.QuickfixView != nil {
		editorCfg.QuickfixView = *cfg.Editor.QuickfixView
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
	if cfg.Editor.SameStatuslineColor != nil {
		editorCfg.SameStatuslineColor = *cfg.Editor.SameStatuslineColor
	}
	if cfg.Editor.StatuslineStyle != nil {
		editorCfg.StatuslineStyle = *cfg.Editor.StatuslineStyle
	}
	commands.NotifyOnSave = false
	commands.GitAiCommit = false
	commands.GitAiTool = "git-ai --tool"
	if cfg.Editor.NotifyOnSave != nil {
		commands.NotifyOnSave = *cfg.Editor.NotifyOnSave
	}
	if cfg.Editor.GitAiCommit != nil {
		commands.GitAiCommit = *cfg.Editor.GitAiCommit
	}
	if cfg.Editor.GitAiTool != nil {
		commands.GitAiTool = *cfg.Editor.GitAiTool
	}
	if cfg.Editor.SaveWorkspaces != nil {
		editorCfg.SaveWorkspaces = *cfg.Editor.SaveWorkspaces
	} else if cfg.Editor.Workspaces != nil {
		editorCfg.SaveWorkspaces = *cfg.Editor.Workspaces
	} else if cfg.Editor.Ws != nil {
		editorCfg.SaveWorkspaces = *cfg.Editor.Ws
	}
	resolve := func(name string) any {
		if def, ok := wig.AllCommands[name]; ok {
			return def.Fn
		}
		return nil
	}

	expandToken := func(token string) string {
		t := strings.ToLower(token)
		if t == "<leader>" {
			return editorCfg.Leader
		}
		if t == "<space>" || token == " " {
			return "Space"
		}

		// Handle <c-x>, <a-x>, etc.
		if strings.HasPrefix(t, "<c-") && strings.HasSuffix(t, ">") {
			return "ctrl+" + strings.ToLower(token[3:len(token)-1])
		}
		if strings.HasPrefix(t, "<ctrl-") && strings.HasSuffix(t, ">") {
			return "ctrl+" + strings.ToLower(token[6:len(token)-1])
		}
		if strings.HasPrefix(t, "<a-") && strings.HasSuffix(t, ">") {
			return "alt+" + strings.ToLower(token[3:len(token)-1])
		}
		if strings.HasPrefix(t, "<alt-") && strings.HasSuffix(t, ">") {
			return "alt+" + strings.ToLower(token[5:len(token)-1])
		}
		if strings.HasPrefix(t, "<m-") && strings.HasSuffix(t, ">") {
			return "meta+" + strings.ToLower(token[3:len(token)-1])
		}
		if strings.HasPrefix(t, "<meta-") && strings.HasSuffix(t, ">") {
			return "meta+" + strings.ToLower(token[6:len(token)-1])
		}
		if strings.HasPrefix(t, "<s-") && strings.HasSuffix(t, ">") {
			return "shift+" + strings.ToLower(token[3:len(token)-1])
		}
		if strings.HasPrefix(t, "<shift-") && strings.HasSuffix(t, ">") {
			return "shift+" + strings.ToLower(token[7:len(token)-1])
		}

		// Handle ctrl+x, alt+x, etc. (case-insensitive matching)
		if strings.HasPrefix(t, "ctrl+") {
			return t
		}
		if strings.HasPrefix(t, "alt+") {
			return t
		}
		if strings.HasPrefix(t, "shift+") {
			return t
		}
		if strings.HasPrefix(t, "meta+") {
			return t
		}

		return token
	}

	applyKeys := func(mode wig.Mode, keys map[string]string) {
		for key, cmdName := range keys {
			fn := resolve(cmdName)
			if fn == nil {
				continue
			}
			tokens := tokenizeKey(key)
			for i, t := range tokens {
				tokens[i] = expandToken(t)
			}
			assignSequence(userMap[mode], tokens, fn)
		}
	}

	applyKeys(wig.MODE_NORMAL, cfg.Keys.Normal)
	applyKeys(wig.MODE_INSERT, cfg.Keys.Insert)
	applyKeys(wig.MODE_VISUAL, cfg.Keys.Visual)
	applyKeys(wig.MODE_VISUAL_LINE, cfg.Keys.VisualLine)
	applyKeys(wig.MODE_VISUAL_BLOCK, cfg.Keys.VisualBlock)

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
			"i": wig.MakeTextObjectKeyMap(true, "comment"),
			"a": wig.MakeTextObjectKeyMap(false, "comment"),
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
			"ctrl+]": commands.CmdTagJump,
			"ctrl+t": wig.CmdJumpBack,
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
				"a": wig.MakeTextObjectKeyMap(false, "change"),
				"i": wig.MakeTextObjectKeyMap(true, "change"),
				"f": wig.CmdChangeTo,
				"t": wig.CmdChangeBefore,
			},
			"d": wig.KeyMap{
				"d": wig.CmdDeleteLine,
				"w": wig.CmdDeleteWord,
				"i": wig.MakeTextObjectKeyMap(true, "delete"),
				"a": wig.MakeTextObjectKeyMap(false, "delete"),
				"f": wig.CmdDeleteTo,
				"t": wig.CmdDeleteBefore,
				"$": wig.CmdDeleteEndOfLine,
				"G": wig.CmdDeleteEndOfFile,
			},
			"y": wig.KeyMap{
				"y": wig.CmdYank,
				"$": wig.CmdYankEol,
				"t": wig.CmdYankBeforeChar,
				"f": wig.CmdYankToChar,
				"i": wig.MakeTextObjectKeyMap(true, "yank"),
				"a": wig.MakeTextObjectKeyMap(false, "yank"),
			},
			">": wig.KeyMap{
				">": wig.CmdIndentLine,
			},
			"<": wig.KeyMap{
				"<": wig.CmdUnindentLine,
			},
			"g": wig.KeyMap{
				"g": wig.CmdGotoLine0,
				"f": wig.CmdGotoFile,
				"F": wig.CmdGotoFileOtherWindow,
				"d": commands.CmdGotoDefinition,
				"t": commands.CmdGitHunkExternalTx,
				"O": commands.CmdGotoDefinitionOtherWindow,
				"o": commands.CmdViewDefinitionOtherWindow,
				"n": commands.CmdGotoNextFunction,
				"N": commands.CmdGotoPrevFunction,
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
				"w": wig.KeyMap{
					"w": commands.CmdWorkspaceListPicker,
					"1": wig.CmdWorkspaceSwitch_1,
					"2": wig.CmdWorkspaceSwitch_2,
					"3": wig.CmdWorkspaceSwitch_3,
					"4": wig.CmdWorkspaceSwitch_4,
					"5": wig.CmdWorkspaceSwitch_5,
					"6": wig.CmdWorkspaceSwitch_6,
					"7": wig.CmdWorkspaceSwitch_7,
					"8": wig.CmdWorkspaceSwitch_8,
					"9": wig.CmdWorkspaceSwitch_9,
					"0": wig.CmdWorkspaceSwitch_0,
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
			"Delete": wig.CmdSelectionDelete,
			"/":      ui.CmdSearchPromptInit,
			"y":      wig.CmdYank,
			"p":      wig.CmdYankPut,
			">":      wig.CmdSelectionIndent,
			"<":      wig.CmdSelectionUnindent,
			"c":      wig.CmdSelectionChange,
			"*":      commands.CmdSearchWordUnderCursor,
			"n":      wig.CmdSearchNext,
			"N":      wig.CmdSearchPrev,
			"%":      wig.WithSelection(wig.CmdMatchPair),
			"g": wig.KeyMap{
				"g": wig.WithSelection(wig.CmdGotoLine0),
				"c": wig.CmdToggleComment,
				"n": wig.WithSelection(commands.CmdGotoNextFunction),
				"N": wig.WithSelection(commands.CmdGotoPrevFunction),
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
			"n":      wig.CmdSearchNext,
			"N":      wig.CmdSearchPrev,
			"x":      wig.CmdSelectionDelete,
			"d":      wig.CmdSelectionDelete,
			"Delete": wig.CmdSelectionDelete,
			"/":      ui.CmdSearchPromptInit,
			"y":      wig.CmdYank,
			">":      wig.CmdSelectionIndent,
			"<":      wig.CmdSelectionUnindent,
			"p":      wig.CmdYankPut,
			"%":      wig.WithSelection(wig.CmdMatchPair),
			"g": wig.KeyMap{
				"g": wig.WithSelection(wig.CmdGotoLine0),
				"c": wig.CmdToggleComment,
				"n": wig.WithSelection(commands.CmdGotoNextFunction),
				"N": wig.WithSelection(commands.CmdGotoPrevFunction),
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
			"n":      wig.CmdSearchNext,
			"N":      wig.CmdSearchPrev,
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
			"alt+/":  autocomplete.PathComplete,
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
