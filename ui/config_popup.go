package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firstrow/wig"
)

type configItem struct {
	Name    string
	Value   interface{}
	Options []string
}

type ConfigPopupWidget struct {
	e            *wig.Editor
	keymap       *wig.KeyHandler
	items        []configItem
	active       int
	scrollOffset int
}

func (u *ConfigPopupWidget) Plane() wig.RenderPlane  { return wig.PlaneEditor }
func (u *ConfigPopupWidget) Mode() wig.Mode          { return wig.MODE_NORMAL }
func (u *ConfigPopupWidget) Keymap() *wig.KeyHandler { return u.keymap }

func ConfigPopupInit(ctx wig.Context) {
	cfg := &ctx.Editor.Config

	widget := &ConfigPopupWidget{
		e:      ctx.Editor,
		active: 0,
		items: []configItem{
			{Name: "show_line_numbers", Value: &cfg.ShowLineNumbers},
			{Name: "relative_line_numbers", Value: &cfg.RelativeLineNumbers},
			{Name: "current_line_absolute", Value: &cfg.CurrentLineAbsolute},
			{Name: "format_on_save", Value: &cfg.FormatOnSave},
			{Name: "indent_guides", Value: &cfg.IndentGuides},
			{Name: "lsp_enabled", Value: &cfg.LspEnabled},
			{Name: "same_statusline_color", Value: &cfg.SameStatuslineColor},
			{Name: "save_workspaces", Value: &cfg.SaveWorkspaces},
			{Name: "statusline_style", Value: &cfg.StatuslineStyle, Options: []string{"plain", "powerline", "airline"}},
			{Name: "gc_motion", Value: &cfg.CommentStyle, Options: []string{"true", "false"}},
			{Name: "which_key_format", Value: &cfg.WhichKeyFormat, Options: []string{"words", "cmd", "camelcase"}},
			{Name: "git_status_view", Value: &cfg.GitStatusView, Options: []string{"full", "split"}},
			{Name: "git_blame_view", Value: &cfg.GitBlameView, Options: []string{"split", "full"}},
			{Name: "quickfix_view", Value: &cfg.QuickfixView, Options: []string{"split", "popup"}},
		},
	}

	km := wig.KeyMap{
		"Esc": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"q": func(ctx wig.Context) {
			ctx.Editor.PopUi()
		},
		"j": func(ctx wig.Context) {
			widget.moveDown(1)
		},
		"Down": func(ctx wig.Context) {
			widget.moveDown(1)
		},
		"k": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Up": func(ctx wig.Context) {
			widget.moveUp(1)
		},
		"Space": func(ctx wig.Context) {
			widget.toggleValue()
		},
		"Enter": func(ctx wig.Context) {
			widget.toggleValue()
		},
		"a": func(ctx wig.Context) {
			widget.saveConfig()
		},
	}

	widget.keymap = wig.NewKeyHandler(wig.ModeKeyMap{wig.MODE_NORMAL: km})
	ctx.Editor.PushUi(widget)
}

func (u *ConfigPopupWidget) moveDown(n int) {
	u.active = min(u.active+n, len(u.items)-1)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *ConfigPopupWidget) moveUp(n int) {
	u.active = max(u.active-n, 0)
	u.ensureVisible()
	u.e.Redraw()
}

func (u *ConfigPopupWidget) ensureVisible() {
	maxVis := 10
	if u.active < u.scrollOffset {
		u.scrollOffset = u.active
	} else if u.active >= u.scrollOffset+maxVis {
		u.scrollOffset = u.active - maxVis + 1
	}
}

func (u *ConfigPopupWidget) saveConfig() {
	cfg := u.e.Config

	home, err := os.UserHomeDir()
	if err != nil {
		u.e.EchoMessage("Error saving config: " + err.Error())
		return
	}

	configDir := filepath.Join(home, ".config", "wig")
	configPath := filepath.Join(configDir, "config.toml")

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}

	values := map[string]string{
		"show_line_numbers":     fmt.Sprintf("%v", cfg.ShowLineNumbers),
		"relative_line_numbers": fmt.Sprintf("%v", cfg.RelativeLineNumbers),
		"current_line_absolute": fmt.Sprintf("%v", cfg.CurrentLineAbsolute),
		"format_on_save":        fmt.Sprintf("%v", cfg.FormatOnSave),
		"indent_guides":         fmt.Sprintf("%v", cfg.IndentGuides),
		"lsp_enabled":           fmt.Sprintf("%v", cfg.LspEnabled),
		"same_statusline_color": fmt.Sprintf("%v", cfg.SameStatuslineColor),
		"save_workspaces":       fmt.Sprintf("%v", cfg.SaveWorkspaces),
		"statusline_style":      fmt.Sprintf("%q", cfg.StatuslineStyle),
		"which_key_format":      fmt.Sprintf("%q", cfg.WhichKeyFormat),
		"git_status_view":       fmt.Sprintf("%q", cfg.GitStatusView),
		"git_blame_view":        fmt.Sprintf("%q", cfg.GitBlameView),
		"quickfix_view":         fmt.Sprintf("%q", cfg.QuickfixView),
		"comment_style":         fmt.Sprintf("%q", cfg.CommentStyle),
	}

	err = updateConfigFile(configPath, values)
	if err != nil {
		u.e.EchoMessage("Error saving config: " + err.Error())
		return
	}

	u.e.EchoMessage("Config updated in config.toml")
	u.e.PopUi()
}

// updateConfigFile preserves comments and formatting by doing a line-by-line
// update of known keys. It drops the legacy `gc_motion` setting if it exists,
// to prevent it from overriding `comment_style`.
func updateConfigFile(path string, values map[string]string) error {
	content := []byte{}
	if data, err := os.ReadFile(path); err == nil {
		content = data
	}

	lines := strings.Split(string(content), "\n")

	editorStart := -1
	editorEnd := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "[editor]" {
			editorStart = i
		} else if strings.HasPrefix(strings.TrimSpace(line), "[") && editorStart != -1 && i > editorStart {
			editorEnd = i
			break
		}
	}

	if editorStart == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "[editor]")
		editorStart = len(lines) - 1
		editorEnd = len(lines)
	}

	updated := make(map[string]bool)
	var newLines []string

	for i, line := range lines {
		if i > editorStart && i < editorEnd {
			trimmed := strings.TrimSpace(line)
			// Remove legacy gc_motion to prevent conflict with comment_style
			if strings.HasPrefix(trimmed, "gc_motion") {
				continue
			}
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				newLines = append(newLines, line)
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				if val, ok := values[key]; ok {
					rightSide := parts[1]
					commentIdx := strings.Index(rightSide, "#")
					if commentIdx >= 0 {
						newLines = append(newLines, fmt.Sprintf("%s = %s %s", key, val, rightSide[commentIdx:]))
					} else {
						newLines = append(newLines, fmt.Sprintf("%s = %s", key, val))
					}
					updated[key] = true
					continue
				}
			}
		}
		newLines = append(newLines, line)
	}

	lines = newLines
	// Recalculate editorEnd in case we removed gc_motion
	editorEnd = len(lines)
	for i, line := range lines {
		if i > editorStart {
			if strings.HasPrefix(strings.TrimSpace(line), "[") {
				editorEnd = i
				break
			}
		}
	}

	// Insert missing keys at the end of the [editor] section
	missing := []string{}
	for key, val := range values {
		if !updated[key] {
			missing = append(missing, fmt.Sprintf("%s = %s", key, val))
		}
	}

	if len(missing) > 0 {
		finalLines := make([]string, 0, len(lines)+len(missing))
		finalLines = append(finalLines, lines[:editorEnd]...)
		finalLines = append(finalLines, missing...)
		finalLines = append(finalLines, lines[editorEnd:]...)
		lines = finalLines
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func (u *ConfigPopupWidget) toggleValue() {
	item := &u.items[u.active]
	switch v := item.Value.(type) {
	case *bool:
		*v = !*v
	case *string:
		if item.Name == "gc_motion" {
			if *v == "standard" {
				*v = "simple"
			} else {
				*v = "standard"
			}
		} else if len(item.Options) > 0 {
			currentIdx := 0
			for i, opt := range item.Options {
				if opt == *v {
					currentIdx = i
					break
				}
			}
			nextIdx := (currentIdx + 1) % len(item.Options)
			*v = item.Options[nextIdx]
		}
	}
	u.e.Redraw()
}

func (u *ConfigPopupWidget) Render(view wig.View) {
	vw, vh := view.Size()

	boxW := int(float32(vw) * 0.9)
	if boxW > vw {
		boxW = vw
	}
	boxH := int(float32(vh) * 0.9)
	if boxH > vh {
		boxH = vh
	}

	maxItems := boxH - 4
	if maxItems < 1 {
		maxItems = 1
	}
	visCount := len(u.items)
	if visCount > maxItems {
		visCount = maxItems
	}

	x := (vw - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (vh - boxH) / 2
	if y < 0 {
		y = 0
	}

	style := wig.Color("default")
	drawBox(view, x, y, boxW, boxH, style)
	view.SetContent(x+2, y, " Config ", wig.Color("ui.popup.title"))

	endIdx := min(u.scrollOffset+visCount, len(u.items))
	for i, item := range u.items[u.scrollOffset:endIdx] {
		row := y + 2 + i
		if row >= y+boxH-1 {
			break
		}

		cursorPrefix := "  "
		if u.scrollOffset+i == u.active {
			cursorPrefix = "> "
		}
		view.SetContent(x+1, row, cursorPrefix, style)

		nameStyle := wig.Color("default")
		if u.scrollOffset+i == u.active {
			nameStyle = wig.Color("ui.menu.selected")
		}
		view.SetContent(x+3, row, item.Name, nameStyle)

		valStr := ""
		switch v := item.Value.(type) {
		case *bool:
			valStr = fmt.Sprintf("%v", *v)
		case *string:
			valStr = *v
			if item.Name == "gc_motion" {
				if *v == "standard" {
					valStr = "true"
				} else {
					valStr = "false"
				}
			}
		}

		valLen := len(valStr)
		valX := x + boxW - 2 - valLen
		view.SetContent(valX, row, valStr, nameStyle)
	}

	view.SetContent(x+2, y+boxH-1, " [Space/Enter] toggle/cycle [a] apply [Esc/q] close ", wig.Color("ui.linenr"))
}
