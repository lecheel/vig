package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// StatuslineData aggregates all buffer, editor, and workspace information
// needed to render any statusline style in a single pass.
type StatuslineData struct {
	Mode        wig.Mode
	ModeName    string
	BufName     string
	IsDirty     bool
	Macro       string
	Message     string
	Line        int
	Char        int
	Scope       string
	Filetype    string
	BufIdx      int
	BufTotal    int
	KeyCount    int
	GitBranch   string
	HasBranch   bool
	IsActive    bool
	SessionName string
}

func extractStatuslineData(e *wig.Editor, win *wig.Window) *StatuslineData {
	buf := win.Buffer()
	if buf == nil {
		return nil
	}

	active := win == e.ActiveWindow()
	cur := wig.CursorGet(e, buf)

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}

	var branch string
	var hasBranch bool
	if wig.GitBranchProvider != nil {
		branch, hasBranch = wig.GitBranchProvider(buf)
	}

	var msg string
	if (active || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		msg = e.Message
	}

	var macro string
	if e.Keys.Macros.Recording() {
		macro = "recording @" + e.Keys.Macros.Register
	}

	var sessionName string
	if ws := e.GetActiveWorkspace(); ws != nil {
		sessionName = ws.ActiveSession
	}

	return &StatuslineData{
		Mode:        buf.Mode(),
		ModeName:    strings.ToUpper(buf.Mode().String()),
		BufName:     buf.GetName(),
		IsDirty:     buf.Dirty,
		Macro:       macro,
		Message:     msg,
		Line:        cur.Line + 1,
		Char:        cur.Char + 1,
		Scope:       trimScope(funcName),
		Filetype:    detectFiletypeLabel(buf.GetName()),
		BufIdx:      bufferIndex(e, buf),
		BufTotal:    len(e.Buffers),
		KeyCount:    e.Keys.GetCount(),
		GitBranch:   branch,
		HasBranch:   hasBranch,
		IsActive:    active,
		SessionName: sessionName,
	}
}

func StatuslineRender(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
) {
	d := extractStatuslineData(e, win)
	if d == nil {
		return
	}
	w, h := view.Size()
	h -= 1
	if h < 0 {
		return
	}

	stActive := wig.Color("ui.statusline")
	stInactive := wig.Color("ui.statusline.inactive")
	stInsert := wig.Color("ui.statusline.insert")

	st := stInactive
	if d.IsActive {
		st = stActive
		if !e.Config.SameStatuslineColor && d.Mode == wig.MODE_INSERT {
			st = stInsert
		}
	}

	// Render EchoMessage 1 line above the statusbar so full statusline info stays visible
	if d.IsActive && d.Message != "" && h-1 >= 0 {
		msgStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		if s, ok := wig.FindColor("ui.message"); ok {
			msgStyle = s
		}
		view.SetContent(0, h-1, strings.Repeat(" ", w), tcell.StyleDefault)
		view.SetContent(0, h-1, d.Message, msgStyle)
	}

	if e.Config.StatuslineStyle == "powerline" {
		renderPowerline(e, view, d, w, h)
	} else {
		renderPlain(e, view, d, st, w, h)
	}
}

func renderPlain(
	e *wig.Editor,
	view wig.View,
	d *StatuslineData,
	st tcell.Style,
	w int,
	h int,
) {
	view.SetContent(0, h, strings.Repeat(" ", w), st)

	macroStr := ""
	if d.Macro != "" {
		macroStr = " " + d.Macro
	}
	leftSide := fmt.Sprintf("%s %s%s ", d.ModeName, d.BufName, macroStr)
	if d.HasBranch {
		leftSide += fmt.Sprintf("  %s ", d.GitBranch)
	}
	view.SetContent(2, h, leftSide, st)

	rightSide := fmt.Sprintf("%d:%d", d.Line, d.Char)
	if d.SessionName != "" {
		rightSide = fmt.Sprintf("[%s]  %s", d.SessionName, rightSide)
	}
	if d.Scope != "" {
		scope := d.Scope
		if len(scope) > 30 {
			scope = scope[:27] + "..."
		}
		rightSide = fmt.Sprintf("%s  %s", scope, rightSide)
	}
	if d.KeyCount > 1 {
		rightSide = fmt.Sprintf("%d   %s", d.KeyCount, rightSide)
	}

	visWidth := runewidth.StringWidth(rightSide)
	if w-visWidth-1 >= 0 {
		view.SetContent(w-visWidth-1, h, rightSide, st)
	}
}

const plMaxScope = 24

func plColor(key string, fallbackBg, fallbackFg tcell.Color) (bg, fg tcell.Color) {
	if s, ok := wig.FindColor(key); ok {
		f, b, _ := s.Decompose()
		if b != tcell.ColorDefault {
			fallbackBg = b
		}
		if f != tcell.ColorDefault {
			fallbackFg = f
		}
	}
	return fallbackBg, fallbackFg
}

func plModeColors(mode wig.Mode) (bg, fg tcell.Color) {
	var key string
	var fallbackBg, fallbackFg tcell.Color

	switch mode {
	case wig.MODE_INSERT:
		key = "ui.statusline.powerline.insert"
		fallbackBg, fallbackFg = tcell.NewRGBColor(0x3a, 0x6b, 0x1f), tcell.ColorWhite
		if s, ok := wig.FindColor("ui.statusline.insert"); ok {
			f, b, _ := s.Decompose()
			if b != tcell.ColorDefault {
				fallbackBg = b
			}
			if f != tcell.ColorDefault {
				fallbackFg = f
			}
		}
	case wig.MODE_VISUAL, wig.MODE_VISUAL_LINE, wig.MODE_VISUAL_BLOCK:
		key = "ui.statusline.powerline.visual"
		fallbackBg, fallbackFg = tcell.NewRGBColor(0x5c, 0x3f, 0x7d), tcell.ColorWhite
		if s, ok := wig.FindColor("ui.statusline.select"); ok {
			f, b, _ := s.Decompose()
			if b != tcell.ColorDefault {
				fallbackBg = b
			}
			if f != tcell.ColorDefault {
				fallbackFg = f
			}
		}
	default:
		key = "ui.statusline.powerline.normal"
		fallbackBg, fallbackFg = tcell.NewRGBColor(0x00, 0x2b, 0x50), tcell.ColorWhite
		if s, ok := wig.FindColor("ui.statusline"); ok {
			f, b, _ := s.Decompose()
			if b != tcell.ColorDefault {
				fallbackBg = b
			}
			if f != tcell.ColorDefault {
				fallbackFg = f
			}
		}
	}

	return plColor(key, fallbackBg, fallbackFg)
}

func trimScope(scope string) string {
	if runewidth.StringWidth(scope) <= plMaxScope {
		return scope
	}
	if parts := strings.Split(scope, "::"); len(parts) > 1 {
		if last := parts[len(parts)-1]; runewidth.StringWidth(last) <= plMaxScope {
			return last
		}
	}
	if parts := strings.Split(scope, "."); len(parts) > 1 {
		if last := parts[len(parts)-1]; runewidth.StringWidth(last) <= plMaxScope {
			return last
		}
	}
	r := []rune(scope)
	if len(r) > plMaxScope {
		r = r[:plMaxScope]
	}
	return string(r)
}

func bufferIndex(e *wig.Editor, buf *wig.Buffer) int {
	for i, b := range e.Buffers {
		if b == buf {
			return i + 1
		}
	}
	return 0
}

func renderPowerline(
	e *wig.Editor,
	view wig.View,
	d *StatuslineData,
	w int,
	h int,
) {
	fallbackFillBg := tcell.NewRGBColor(0, 122, 204)
	if s, ok := wig.FindColor("ui.statusline"); ok {
		_, b, _ := s.Decompose()
		if b != tcell.ColorDefault {
			fallbackFillBg = b
		}
	}
	bgFill, _ := plColor("ui.statusline.powerline.fill", fallbackFillBg, tcell.ColorWhite)
	if !d.IsActive {
		_, inactiveBg, _ := wig.Color("ui.statusline.inactive").Decompose()
		bgFill = inactiveBg
	}
	view.SetContent(0, h, strings.Repeat(" ", w), tcell.StyleDefault.Background(bgFill))

	type segment struct {
		text string
		fg   tcell.Color
		bg   tcell.Color
	}

	modeBg, modeFg := plModeColors(d.Mode)
	if !d.IsActive {
		modeBg, modeFg = bgFill, tcell.ColorSilver
	}

	var leftSegs []segment
	leftSegs = append(leftSegs, segment{
		text: fmt.Sprintf(" %s ", d.ModeName),
		fg:   modeFg, bg: modeBg,
	})

	if d.HasBranch {
		branchBg, branchFg := plColor("ui.statusline.powerline.branch", tcell.NewRGBColor(9, 71, 113), tcell.ColorWhite)
		if !d.IsActive {
			branchBg = bgFill
		}
		leftSegs = append(leftSegs, segment{
			text: fmt.Sprintf("  %s ", d.GitBranch),
			fg:   branchFg, bg: branchBg,
		})
	}

	nameText := d.BufName
	if d.IsDirty {
		nameText += " [+]"
	}
	fileBg, fileFg := plColor("ui.statusline.powerline.file", tcell.NewRGBColor(38, 79, 120), tcell.NewRGBColor(230, 230, 230))
	if !d.IsActive {
		fileBg = bgFill
	}
	leftSegs = append(leftSegs, segment{
		text: fmt.Sprintf(" %s ", nameText),
		fg:   fileFg, bg: fileBg,
	})

	if d.Macro != "" {
		leftSegs = append(leftSegs, segment{
			text: fmt.Sprintf(" REC @%s ", strings.TrimPrefix(d.Macro, "recording @")),
			fg:   tcell.ColorWhite, bg: bgFill,
		})
	}

	var rightSegs []segment

	if d.Scope != "" {
		scopeBg, scopeFg := plColor("ui.statusline.powerline.scope", tcell.NewRGBColor(38, 79, 120), tcell.NewRGBColor(210, 210, 210))
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %s ", d.Scope),
			fg:   scopeFg, bg: scopeBg,
		})
	}

	if d.BufTotal > 1 {
		bufBg, bufFg := plColor("ui.statusline.powerline.buf", tcell.NewRGBColor(27, 129, 168), tcell.NewRGBColor(220, 220, 235))
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d/%d ", d.BufIdx, d.BufTotal),
			fg:   bufFg, bg: bufBg,
		})
	}

	if d.KeyCount > 1 {
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d ", d.KeyCount),
			fg:   tcell.ColorWhite, bg: bgFill,
		})
	}

	langBg, langFg := plColor("ui.statusline.powerline.lang", tcell.NewRGBColor(9, 71, 113), tcell.ColorWhite)
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %s ", d.Filetype),
		fg:   langFg, bg: langBg,
	})

	posBg, posFg := plColor("ui.statusline.powerline.pos", tcell.NewRGBColor(0, 122, 204), tcell.ColorWhite)
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %4d:%-3d", d.Line, d.Char),
		fg:   posFg, bg: posBg,
	})

	if d.SessionName != "" {
		sesBg, sesFg := plColor("ui.statusline.powerline.session", tcell.NewRGBColor(0x6a, 0x4c, 0x93), tcell.ColorWhite)
		if !d.IsActive {
			sesBg = bgFill
		}
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %s  ", d.SessionName),
			fg:   sesFg, bg: sesBg,
		})
	}

	arrowL := "\ue0b0" // points right
	arrowR := "\ue0b2" // points left
	aw := runewidth.StringWidth(arrowL)
	if aw == 0 {
		aw = 1
	}

	// Draw left side.
	x := 0
	for i, s := range leftSegs {
		if i > 0 {
			prevBg := leftSegs[i-1].bg
			arrowStyle := tcell.StyleDefault.Background(s.bg).Foreground(prevBg)
			view.SetContent(x, h, arrowL, arrowStyle)
			x += aw
		}

		view.SetContent(x, h, s.text, tcell.StyleDefault.Background(s.bg).Foreground(s.fg).Bold(true))
		x += runewidth.StringWidth(s.text)
	}
	if len(leftSegs) > 0 {
		lastBg := leftSegs[len(leftSegs)-1].bg
		arrowStyle := tcell.StyleDefault.Background(bgFill).Foreground(lastBg)
		view.SetContent(x, h, arrowL, arrowStyle)
		x += aw
	}

	// Draw right side leftward from the screen's right edge.
	x = w
	for i := len(rightSegs) - 1; i >= 0; i-- {
		s := rightSegs[i]
		textWidth := runewidth.StringWidth(s.text)

		view.SetContent(x-textWidth, h, s.text, tcell.StyleDefault.Background(s.bg).Foreground(s.fg).Bold(true))
		x -= textWidth

		if i > 0 {
			prevBg := rightSegs[i-1].bg
			arrowStyle := tcell.StyleDefault.Background(prevBg).Foreground(s.bg)
			view.SetContent(x-aw, h, arrowR, arrowStyle)
			x -= aw
		} else {
			arrowStyle := tcell.StyleDefault.Background(bgFill).Foreground(s.bg)
			view.SetContent(x-aw, h, arrowR, arrowStyle)
			x -= aw
		}
	}
}

// detectFiletypeLabel returns a short human-readable filetype label based
// on the buffer's filename extension, used in statusline info boxes
// (e.g. "Go", "Plain Text").
func detectFiletypeLabel(name string) string {
	switch {
	case strings.HasSuffix(name, ".go"):
		return "Go"
	case strings.HasSuffix(name, ".rs"):
		return "Rust"
	case strings.HasSuffix(name, ".py"):
		return "Python"
	case strings.HasSuffix(name, ".c"), strings.HasSuffix(name, ".h"):
		return "C"
	case strings.HasSuffix(name, ".json"), strings.HasSuffix(name, ".jsonc"):
		return "JSON"
	case strings.HasSuffix(name, ".toml"):
		return "TOML"
	case strings.HasSuffix(name, ".sh"), strings.HasSuffix(name, ".bash"), strings.HasSuffix(name, ".zsh"):
		return "Shell"
	case strings.HasSuffix(name, ".md"):
		return "Markdown"
	default:
		return "Plain Text"
	}
}
