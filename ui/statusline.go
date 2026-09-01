package ui

import (
	"fmt"
	"strings"

	"github.com/firstrow/wig"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func StatuslineRender(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
) {
	buf := win.Buffer()
	if buf == nil {
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
	if win == e.ActiveWindow() {
		st = stActive
		if !e.Config.SameStatuslineColor && buf.Mode() == wig.MODE_INSERT {
			st = stInsert
		}
	}

	if e.Config.StatuslineStyle == "powerline" {
		renderPowerline(e, view, win, st, w, h)
	} else if e.Config.StatuslineStyle == "airline" {
		renderAirline(e, view, win, w, h)
	} else {
		renderPlain(e, view, win, st, stInactive, w, h)
	}
}

func renderPlain(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
	st tcell.Style,
	stInactive tcell.Style,
	w int,
	h int,
) {
	buf := win.Buffer()

	bg := strings.Repeat(" ", w)
	view.SetContent(0, h, bg, st)

	macroStatus := ""
	if e.Keys.Macros.Recording() {
		macroStatus = "recording @" + e.Keys.Macros.Register
	}
	leftSide := fmt.Sprintf("%s %s %s ", buf.Mode().String(), buf.GetName(), macroStatus)
	if (win == e.ActiveWindow() || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		leftSide = e.Message
	}
	view.SetContent(2, h, leftSide, st)

	cur := wig.CursorGet(e, buf)

	wsIndicator := "ð"
	if e.Config.SaveWorkspaces {
		wsIndicator = "ð"
	}

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}
	rightSide := fmt.Sprintf("%s[ws:%d] %d:%d", wsIndicator, e.ActiveWorkspace, cur.Line+1, cur.Char)
	if funcName != "" {
		if len(funcName) > 30 {
			funcName = funcName[:27] + "..."
		}
		rightSide = fmt.Sprintf("%s  %s", funcName, rightSide)
	}
	if e.Keys.GetCount() > 1 {
		rightSide = fmt.Sprintf("%d   %s", e.Keys.GetCount(), rightSide)
	}

	visWidth := runewidth.StringWidth(rightSide)
	if w-visWidth-1 >= 0 {
		view.SetContent(w-visWidth-1, h, rightSide, st)
	}
}

// Powerline palette, ported from the reference statusbar.rs implementation.
// Each color is a distinct dark-slate tone so segments read clearly against
// each other without relying on terminal-defined ANSI colors.
var (
	plBgFill   = tcell.NewRGBColor(36, 36, 44) // deep slate base (gap fill)
	plBgFile   = tcell.NewRGBColor(50, 50, 60) // slightly lighter slate
	plBranchBg = tcell.NewRGBColor(45, 70, 55) // muted forest green
	plRPos     = tcell.NewRGBColor(60, 70, 95) // dark steel blue
	plRScope   = tcell.NewRGBColor(55, 55, 68) // muted purple-gray
	plRLang    = tcell.NewRGBColor(45, 65, 75) // dark teal
	plRBuf     = tcell.NewRGBColor(65, 55, 80) // dark indigo
)

const plMaxScope = 24

// plModeColors returns the (bg, fg) pair for the leftmost mode segment,
// mirroring the reference implementation's per-mode palette. wig's Mode
// type doesn't distinguish Command/Search/Brief/LlmPrompt the way the
// reference editor does, so those collapse onto the closest wig mode.
func plModeColors(mode wig.Mode) (bg, fg tcell.Color) {
	switch mode {
	case wig.MODE_INSERT:
		return tcell.NewRGBColor(45, 75, 55), tcell.NewRGBColor(220, 245, 230)
	case wig.MODE_VISUAL, wig.MODE_VISUAL_LINE, wig.MODE_VISUAL_BLOCK:
		return tcell.NewRGBColor(100, 65, 45), tcell.NewRGBColor(245, 230, 220)
	default: // wig.MODE_NORMAL
		return tcell.NewRGBColor(40, 70, 85), tcell.NewRGBColor(220, 235, 245)
	}
}

// trimScope shortens a function/scope name to at most plMaxScope runes,
// preferring the last "::" or "." component (e.g. "impl Foo::bar" -> "bar")
// before falling back to a hard truncate — ported from the reference
// implementation's trim_scope.
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

// bufferIndex returns the 1-based position of buf within e.Buffers, or 0
// if not found — used for the "N/total" buffer-count segment.
func bufferIndex(e *wig.Editor, buf *wig.Buffer) int {
	for i, b := range e.Buffers {
		if b == buf {
			return i + 1
		}
	}
	return 0
}

// renderPowerline draws a richly-segmented powerline statusline, ported
// from the reference statusbar.rs: a per-mode colored mode segment,
// filename (with a "[+]" modified marker), then on the right a fixed-width
// position, LSP/AI status, filetype, enclosing function scope, and buffer
// count — each segment chained with a triangle arrow whose colors
// transition from the segment it's leaving into the segment (or fill) it's
// entering, matching left_span/right_span in the reference.
//
// NOTE: a real git branch/diff-stat segment (as in the reference) isn't
// wired in here because that data lives in the commands package, and
// commands already imports ui (see commands/git_view.go, git_hunk.go),
// so ui importing commands back would create an import cycle. Exposing
// git status on wig.Editor itself (populated by commands, read by ui)
// would let this segment be added without that cycle.
func renderPowerline(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
	st tcell.Style,
	w int,
	h int,
) {
	buf := win.Buffer()
	active := win == e.ActiveWindow()

	bgFill := plBgFill
	if !active {
		_, inactiveBg, _ := wig.Color("ui.statusline.inactive").Decompose()
		bgFill = inactiveBg
	}
	view.SetContent(0, h, strings.Repeat(" ", w), tcell.StyleDefault.Background(bgFill))

	type segment struct {
		text string
		fg   tcell.Color
		bg   tcell.Color
	}

	modeBg, modeFg := plModeColors(buf.Mode())
	if !active {
		modeBg, modeFg = bgFill, tcell.ColorSilver
	}

	var leftSegs []segment
	leftSegs = append(leftSegs, segment{
		text: fmt.Sprintf(" %s ", strings.ToUpper(buf.Mode().String())),
		fg:   modeFg, bg: modeBg,
	})

	if wig.GitBranchProvider != nil {
		if branch, ok := wig.GitBranchProvider(buf); ok {
			branchBg := plBranchBg
			if !active {
				branchBg = bgFill
			}
			leftSegs = append(leftSegs, segment{
				text: fmt.Sprintf(" %s ", branch),
				fg:   tcell.NewRGBColor(215, 235, 220), bg: branchBg,
			})
		}
	}

	nameText := buf.GetName()
	if buf.Dirty {
		nameText += " [+]"
	}
	fileBg := plBgFile
	if !active {
		fileBg = bgFill
	}
	leftSegs = append(leftSegs, segment{
		text: fmt.Sprintf(" %s ", nameText),
		fg:   tcell.NewRGBColor(210, 215, 225), bg: fileBg,
	})

	if e.Keys.Macros.Recording() {
		leftSegs = append(leftSegs, segment{
			text: fmt.Sprintf(" REC @%s ", e.Keys.Macros.Register),
			fg:   tcell.ColorWhite, bg: bgFill,
		})
	}

	if (active || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		leftSegs = []segment{{text: fmt.Sprintf(" %s ", e.Message), fg: tcell.ColorWhite, bg: bgFill}}
	}

	cur := wig.CursorGet(e, buf)

	// Right side, built in final left-to-right visual order: optional
	// scope/buffer-count segments, then filetype, then position last so
	// it always sits at the far right edge (no AI segment anymore).
	var rightSegs []segment

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}
	if funcName != "" {
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %s ", trimScope(funcName)),
			fg:   tcell.NewRGBColor(180, 180, 195), bg: plRScope,
		})
	}

	if len(e.Buffers) > 1 {
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d/%d ", bufferIndex(e, buf), len(e.Buffers)),
			fg:   tcell.NewRGBColor(210, 205, 230), bg: plRBuf,
		})
	}

	if e.Keys.GetCount() > 1 {
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d ", e.Keys.GetCount()),
			fg:   tcell.ColorWhite, bg: bgFill,
		})
	}

	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %s ", detectFiletypeLabel(buf.GetName())),
		fg:   tcell.NewRGBColor(200, 220, 225), bg: plRLang,
	})

	// Rightmost: fixed-width line:col, so the position doesn't jitter the
	// segments to its left as digit counts change.
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %4d:%-3d", cur.Line+1, cur.Char+1),
		fg:   tcell.NewRGBColor(200, 210, 230), bg: plRPos,
	})

	arrowL := "\ue0b0" // points right: leaving-segment fg -> entering-segment bg
	arrowR := "\ue0b2" // points left: entering-segment fg -> leaving-segment bg
	aw := runewidth.StringWidth(arrowL)
	if aw == 0 {
		aw = 1
	}

	// Draw left side. The first segment starts flush against the left
	// edge with no leading arrow — there's nothing before it to
	// transition from, so a cap there would just be a triangle pointing
	// out of the plain background into itself. Arrows only appear
	// between two real segments.
	x := 0
	for i, s := range leftSegs {
		if i > 0 {
			prevBg := leftSegs[i-1].bg
			arrowStyle := tcell.StyleDefault.Background(s.bg).Foreground(prevBg).Bold(true)
			view.SetContent(x, h, arrowL, arrowStyle)
			x += aw
		}

		view.SetContent(x, h, s.text, tcell.StyleDefault.Background(s.bg).Foreground(s.fg).Bold(true))
		x += runewidth.StringWidth(s.text)
	}

	// Draw right side leftward from the screen's right edge. Same rule
	// mirrored: the last segment (rightmost, position) ends flush with
	// no trailing arrow past it into the plain background.
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
		}
	}
}

// airlineModeStyle returns the mode-specific background/foreground style
// for the leading "NORMAL"/"INSERT"/"VISUAL" box, mimicking vim-airline's
// per-mode coloring. Themes can override any of these via
// "ui.statusline.airline.normal" / ".insert" / ".visual"; otherwise a
// sensible hardcoded fallback color is used.
func airlineModeStyle(mode wig.Mode) tcell.Style {
	var key string
	var fallbackBg tcell.Color

	switch mode {
	case wig.MODE_INSERT:
		key = "ui.statusline.airline.insert"
		fallbackBg = tcell.NewRGBColor(0x5f, 0x87, 0x00)
	case wig.MODE_VISUAL, wig.MODE_VISUAL_LINE, wig.MODE_VISUAL_BLOCK:
		key = "ui.statusline.airline.visual"
		fallbackBg = tcell.NewRGBColor(0x87, 0x5f, 0x87)
	default:
		key = "ui.statusline.airline.normal"
		fallbackBg = tcell.NewRGBColor(0x00, 0x5f, 0x87)
	}

	if s, ok := wig.FindColor(key); ok {
		return s
	}
	return tcell.StyleDefault.Background(fallbackBg).Foreground(tcell.ColorWhite)
}

// renderAirline draws a flat, boxed statusline (no powerline triangles):
// a per-mode colored box on the far left, a slightly lighter box for the
// buffer name, and thin-divider-separated boxes on the right for
// filetype/AI status and the cursor position — similar to a minimal
// VS Code / Zed style statusbar rather than classic vim-airline arrows.
func renderAirline(
	e *wig.Editor,
	view wig.View,
	win *wig.Window,
	w int,
	h int,
) {
	buf := win.Buffer()

	baseSt := wig.Color("ui.statusline.inactive")
	if win == e.ActiveWindow() {
		baseSt = wig.Color("ui.statusline")
	}

	bgFill := strings.Repeat(" ", w)
	view.SetContent(0, h, bgFill, baseSt)

	type segment struct {
		text  string
		style tcell.Style
	}

	modeSt := baseSt
	if win == e.ActiveWindow() {
		modeSt = airlineModeStyle(buf.Mode())
	}

	// A subtly lighter box than the base background, used for the
	// filename and the right-hand info boxes.
	boxSt := baseSt
	if _, ok := wig.FindColor("ui.statusline.airline.box"); ok {
		boxSt = wig.Color("ui.statusline.airline.box")
	}

	var leftSegs []segment
	leftSegs = append(leftSegs, segment{text: fmt.Sprintf(" %s ", strings.ToUpper(buf.Mode().String())), style: modeSt})

	nameText := buf.GetName()
	if buf.Dirty {
		nameText += " +"
	}
	leftSegs = append(leftSegs, segment{text: fmt.Sprintf(" %s ", nameText), style: boxSt})

	if e.Keys.Macros.Recording() {
		leftSegs = append(leftSegs, segment{text: fmt.Sprintf(" REC @%s ", e.Keys.Macros.Register), style: baseSt})
	}

	if (win == e.ActiveWindow() || win.Buffer() == e.ActiveWindow().Buffer()) && len(e.Message) > 0 {
		leftSegs = []segment{{text: fmt.Sprintf(" %s ", e.Message), style: baseSt}}
	}

	cur := wig.CursorGet(e, buf)

	var rightSegs []segment

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}
	if funcName != "" {
		if len(funcName) > 30 {
			funcName = funcName[:27] + "..."
		}
		rightSegs = append(rightSegs, segment{text: fmt.Sprintf(" %s ", funcName), style: baseSt})
	}

	ft := detectFiletypeLabel(buf.GetName())
	rightSegs = append(rightSegs, segment{text: fmt.Sprintf(" %s ", ft), style: baseSt})

	aiStatus := "AI OFF"
	if e.Config.LspEnabled {
		aiStatus = "AI ON"
	}
	rightSegs = append(rightSegs, segment{text: fmt.Sprintf(" %s ", aiStatus), style: baseSt})

	posText := fmt.Sprintf(" %d:%d ", cur.Line+1, cur.Char+1)
	rightSegs = append(rightSegs, segment{text: posText, style: baseSt})

	if e.Keys.GetCount() > 1 {
		rightSegs = append([]segment{{text: fmt.Sprintf(" %d ", e.Keys.GetCount()), style: baseSt}}, rightSegs...)
	}

	// Draw left side: solid boxes, no separators between mode and name.
	x := 0
	for _, s := range leftSegs {
		view.SetContent(x, h, s.text, s.style)
		x += runewidth.StringWidth(s.text)
	}

	// Draw right side: right-aligned boxes separated by vim-airline's
	// "thin" separator glyph (U+E0B1), used between same-colored
	// sections — as opposed to the bold U+E0B0/U+E0B2 triangles used
	// for color-changing transitions in renderPowerline above.
	//
	// NOTE: this glyph lives in the Private Use Area and requires a
	// Nerd Font (or another font patched with powerline symbols) to be
	// selected in the terminal. Without one, it renders as a blank,
	// tofu box, or missing-glyph placeholder — this is a terminal/font
	// configuration issue, not a bug in wig. See:
	// https://discourse.nixos.org/t/airline-powerline-in-neovim-not-working-not-showing-glyphs/14211
	divider := "\ue0b1"
	x = w
	for i := len(rightSegs) - 1; i >= 0; i-- {
		s := rightSegs[i]
		textWidth := runewidth.StringWidth(s.text)
		view.SetContent(x-textWidth, h, s.text, s.style)
		x -= textWidth

		if i > 0 {
			dw := runewidth.StringWidth(divider)
			if dw == 0 {
				dw = 1
			}
			view.SetContent(x-dw, h, divider, baseSt)
			x -= dw
		}
	}
}

// detectFiletypeLabel returns a short human-readable filetype label based
// on the buffer's filename extension, used in the airline-style
// statusline's right-hand info boxes (e.g. "Go", "Plain Text").
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
