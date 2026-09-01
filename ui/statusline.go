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

	wsIndicator := "🔒"
	if e.Config.SaveWorkspaces {
		wsIndicator = "💾"
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

// plModeColors returns the (bg, fg) pair for the leftmost mode segment,
// checking theme-defined powerline colors with fallback to statusline colors.
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

	fallbackFillBg := tcell.NewRGBColor(0, 122, 204)
	if s, ok := wig.FindColor("ui.statusline"); ok {
		_, b, _ := s.Decompose()
		if b != tcell.ColorDefault {
			fallbackFillBg = b
		}
	}
	bgFill, _ := plColor("ui.statusline.powerline.fill", fallbackFillBg, tcell.ColorWhite)
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
			branchBg, branchFg := plColor("ui.statusline.powerline.branch", tcell.NewRGBColor(9, 71, 113), tcell.ColorWhite)
			if !active {
				branchBg = bgFill
			}
			leftSegs = append(leftSegs, segment{
				text: fmt.Sprintf(" %s ", branch),
				fg:   branchFg, bg: branchBg,
			})
		}
	}

	nameText := buf.GetName()
	if buf.Dirty {
		nameText += " [+]"
	}
	fileBg, fileFg := plColor("ui.statusline.powerline.file", tcell.NewRGBColor(38, 79, 120), tcell.NewRGBColor(230, 230, 230))
	if !active {
		fileBg = bgFill
	}
	leftSegs = append(leftSegs, segment{
		text: fmt.Sprintf(" %s ", nameText),
		fg:   fileFg, bg: fileBg,
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

	var rightSegs []segment

	funcName := ""
	if ts, ok := buf.Highlighter.(*wig.TreeSitterHighlighter); ok && ts != nil {
		funcName = ts.FunctionAtLine(cur.Line)
	}
	if funcName != "" {
		scopeBg, scopeFg := plColor("ui.statusline.powerline.scope", tcell.NewRGBColor(38, 79, 120), tcell.NewRGBColor(210, 210, 210))
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %s ", trimScope(funcName)),
			fg:   scopeFg, bg: scopeBg,
		})
	}

	if len(e.Buffers) > 1 {
		bufBg, bufFg := plColor("ui.statusline.powerline.buf", tcell.NewRGBColor(27, 129, 168), tcell.NewRGBColor(220, 220, 235))
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d/%d ", bufferIndex(e, buf), len(e.Buffers)),
			fg:   bufFg, bg: bufBg,
		})
	}

	wsIndicator := "🔒"
	if e.Config.SaveWorkspaces {
		wsIndicator = "💾"
	}
	wsBg, wsFg := plColor("ui.statusline.powerline.ws", tcell.NewRGBColor(38, 79, 120), tcell.NewRGBColor(210, 210, 230))
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %s [ws:%d] ", wsIndicator, e.ActiveWorkspace),
		fg:   wsFg, bg: wsBg,
	})

	if e.Keys.GetCount() > 1 {
		rightSegs = append(rightSegs, segment{
			text: fmt.Sprintf(" %d ", e.Keys.GetCount()),
			fg:   tcell.ColorWhite, bg: bgFill,
		})
	}

	langBg, langFg := plColor("ui.statusline.powerline.lang", tcell.NewRGBColor(9, 71, 113), tcell.ColorWhite)
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %s ", detectFiletypeLabel(buf.GetName())),
		fg:   langFg, bg: langBg,
	})

	posBg, posFg := plColor("ui.statusline.powerline.pos", tcell.NewRGBColor(0, 122, 204), tcell.ColorWhite)
	rightSegs = append(rightSegs, segment{
		text: fmt.Sprintf(" %4d:%-3d", cur.Line+1, cur.Char+1),
		fg:   posFg, bg: posBg,
	})

	arrowL := "\ue0b0" // points right: leaving-segment fg -> entering-segment bg
	arrowR := "\ue0b2" // points left: entering-segment fg -> leaving-segment bg
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
