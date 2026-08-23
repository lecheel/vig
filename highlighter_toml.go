package wig

import (
	"path/filepath"
	"strings"
	"unicode"
)

// NewHighlighterForPath returns a syntax highlighter based on file extension
func NewHighlighterForPath(buf *Buffer, path string) Highlighter {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		return &TomlHighlighter{Buf: buf}
	case ".json", ".jsonc":
		return &JsonHighlighter{Buf: buf}
	}
	return nil
}

// TomlHighlighter provides syntax coloring for TOML files
type TomlHighlighter struct {
	Buf *Buffer
}

func (h *TomlHighlighter) Build()                      {}
func (h *TomlHighlighter) TextChanged(EventTextChange) {}

func (h *TomlHighlighter) ForRange(startLine, endLine uint32) *HighlighterCursor {
	if h.Buf == nil {
		return nil
	}

	nodes := List[HighlighterNode]{}

	line := CursorLineByNum(h.Buf, int(startLine))
	for lineNum := startLine; line != nil && lineNum <= endLine; lineNum++ {
		runes := line.Value
		lineLen := len(runes)
		for lineLen > 0 && (runes[lineLen-1] == '\n' || runes[lineLen-1] == '\r') {
			lineLen--
		}

		if lineLen > 0 {
			h.highlightTomlLine(&nodes, lineNum, runes[:lineLen])
		}

		line = line.Next()
	}

	if nodes.First() == nil {
		return nil
	}
	return &HighlighterCursor{Cursor: nodes.First()}
}

func (h *TomlHighlighter) highlightTomlLine(nodes *List[HighlighterNode], lineNum uint32, runes []rune) {
	n := len(runes)
	i := 0

	// Skip leading whitespace
	for i < n && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	if i >= n {
		return
	}

	// Comment line
	if runes[i] == '#' {
		nodes.PushBack(HighlighterNode{
			NodeName:  "comment",
			StartLine: lineNum,
			StartChar: uint32(i),
			EndLine:   lineNum,
			EndChar:   uint32(n),
		})
		return
	}

	// Table header: [table] or [[array]]
	if runes[i] == '[' {
		// Check if it's a section header (bracket closed, followed only by spaces or comments)
		isHeader := false
		closeIdx := -1
		for j := i + 1; j < n; j++ {
			if runes[j] == ']' {
				closeIdx = j
				if j+1 < n && runes[j+1] == ']' {
					closeIdx = j + 1
				}
				// Verify remainder is only whitespace or comment
				rest := j + 1
				if closeIdx == j+1 {
					rest = j + 2
				}
				validRest := true
				for k := rest; k < n; k++ {
					if runes[k] == '#' {
						break
					}
					if runes[k] != ' ' && runes[k] != '\t' {
						validRest = false
						break
					}
				}
				if validRest {
					isHeader = true
				}
				break
			}
		}

		if isHeader && closeIdx >= 0 {
			nodes.PushBack(HighlighterNode{
				NodeName:  "type",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(closeIdx + 1),
			})

			// Check for trailing comment
			for k := closeIdx + 1; k < n; k++ {
				if runes[k] == '#' {
					nodes.PushBack(HighlighterNode{
						NodeName:  "comment",
						StartLine: lineNum,
						StartChar: uint32(k),
						EndLine:   lineNum,
						EndChar:   uint32(n),
					})
					break
				}
			}
			return
		}
	}

	// Tokenize line: keys, operators, strings, numbers, booleans, comments
	for i < n {
		if runes[i] == ' ' || runes[i] == '\t' {
			i++
			continue
		}

		// Comment
		if runes[i] == '#' {
			nodes.PushBack(HighlighterNode{
				NodeName:  "comment",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(n),
			})
			return
		}

		// String
		if runes[i] == '"' || runes[i] == '\'' {
			start := i
			end := scanString(runes, i)

			// Check if this string is a quoted key (followed by '=')
			isKey := false
			for j := end; j < n; j++ {
				if runes[j] == ' ' || runes[j] == '\t' {
					continue
				}
				if runes[j] == '=' {
					isKey = true
				}
				break
			}

			nodeName := "string"
			if isKey {
				nodeName = "variable.other.member"
			}

			nodes.PushBack(HighlighterNode{
				NodeName:  nodeName,
				StartLine: lineNum,
				StartChar: uint32(start),
				EndLine:   lineNum,
				EndChar:   uint32(end),
			})
			i = end
			continue
		}

		// Operator '='
		if runes[i] == '=' {
			nodes.PushBack(HighlighterNode{
				NodeName:  "operator",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(i + 1),
			})
			i++
			continue
		}

		// Punctuation
		if runes[i] == '[' || runes[i] == ']' || runes[i] == '{' || runes[i] == '}' {
			nodes.PushBack(HighlighterNode{
				NodeName:  "punctuation.bracket",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(i + 1),
			})
			i++
			continue
		}
		if runes[i] == ',' || runes[i] == '.' {
			nodes.PushBack(HighlighterNode{
				NodeName:  "punctuation.delimiter",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(i + 1),
			})
			i++
			continue
		}

		// Identifier / number / boolean / datetime
		start := i
		for i < n && !isTomlDelimiter(runes[i]) {
			i++
		}
		token := string(runes[start:i])

		// Look ahead to check if this token is a key (followed by '=')
		isKey := false
		for j := i; j < n; j++ {
			if runes[j] == ' ' || runes[j] == '\t' {
				continue
			}
			if runes[j] == '=' {
				isKey = true
			}
			break
		}

		var nodeName string
		switch {
		case isKey:
			nodeName = "variable.other.member"
		case token == "true" || token == "false":
			nodeName = "constant.builtin.boolean"
		case isTomlInteger(token):
			nodeName = "constant.numeric.integer"
		case isTomlFloat(token):
			nodeName = "constant.numeric.float"
		case isTomlDateTime(token):
			nodeName = "string.special"
		default:
			nodeName = "constant"
		}

		nodes.PushBack(HighlighterNode{
			NodeName:  nodeName,
			StartLine: lineNum,
			StartChar: uint32(start),
			EndLine:   lineNum,
			EndChar:   uint32(i),
		})
	}
}

func isTomlDelimiter(r rune) bool {
	return r == ' ' || r == '\t' || r == '=' || r == '#' || r == ',' ||
		r == '[' || r == ']' || r == '{' || r == '}' || r == '"' || r == '\''
}

func isTomlInteger(s string) bool {
	if len(s) == 0 {
		return false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") ||
		strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") ||
		strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		return true
	}
	if strings.Contains(s, ".") || strings.Contains(s, "e") || strings.Contains(s, "E") {
		return false
	}
	r := rune(s[0])
	return unicode.IsDigit(r) || ((r == '-' || r == '+') && len(s) > 1 && unicode.IsDigit(rune(s[1])))
}

func isTomlFloat(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s == "inf" || s == "+inf" || s == "-inf" || s == "nan" || s == "+nan" || s == "-nan" {
		return true
	}
	if strings.Contains(s, ".") || strings.Contains(s, "e") || strings.Contains(s, "E") {
		r := rune(s[0])
		return unicode.IsDigit(r) || ((r == '-' || r == '+') && len(s) > 1)
	}
	return false
}

func isTomlDateTime(s string) bool {
	if len(s) >= 8 && s[4] == '-' && s[7] == '-' {
		return true
	}
	if len(s) >= 8 && s[2] == ':' && s[5] == ':' {
		return true
	}
	return false
}

func scanString(runes []rune, idx int) int {
	quote := runes[idx]
	isTriple := false
	if idx+2 < len(runes) && runes[idx+1] == quote && runes[idx+2] == quote {
		isTriple = true
		idx += 3
	} else {
		idx++
	}

	for idx < len(runes) {
		if isTriple {
			if runes[idx] == quote && idx+2 < len(runes) && runes[idx+1] == quote && runes[idx+2] == quote {
				return idx + 3
			}
			idx++
		} else {
			if runes[idx] == '\\' && quote == '"' {
				idx += 2
				continue
			}
			if runes[idx] == quote {
				return idx + 1
			}
			idx++
		}
	}
	return len(runes)
}
