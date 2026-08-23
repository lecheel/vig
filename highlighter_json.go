package wig

import (
	"unicode"
)

// JsonHighlighter provides syntax coloring for JSON files
type JsonHighlighter struct {
	Buf *Buffer
}

func (h *JsonHighlighter) Build()                      {}
func (h *JsonHighlighter) TextChanged(EventTextChange) {}

func (h *JsonHighlighter) ForRange(startLine, endLine uint32) *HighlighterCursor {
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
			h.highlightJsonLine(&nodes, lineNum, runes[:lineLen])
		}

		line = line.Next()
	}

	if nodes.First() == nil {
		return nil
	}
	return &HighlighterCursor{Cursor: nodes.First()}
}

func (h *JsonHighlighter) highlightJsonLine(nodes *List[HighlighterNode], lineNum uint32, runes []rune) {
	n := len(runes)
	i := 0

	for i < n {
		if runes[i] == ' ' || runes[i] == '\t' {
			i++
			continue
		}

		// Comment support (// or #)
		if (runes[i] == '/' && i+1 < n && runes[i+1] == '/') || runes[i] == '#' {
			nodes.PushBack(HighlighterNode{
				NodeName:  "comment",
				StartLine: lineNum,
				StartChar: uint32(i),
				EndLine:   lineNum,
				EndChar:   uint32(n),
			})
			return
		}

		// String (key or value)
		if runes[i] == '"' {
			start := i
			end := scanString(runes, i)

			// Check if this string is an object key (followed by ':')
			isKey := false
			for j := end; j < n; j++ {
				if runes[j] == ' ' || runes[j] == '\t' {
					continue
				}
				if runes[j] == ':' {
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

		// Punctuation & Delimiters
		if runes[i] == ':' {
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
		if runes[i] == ',' {
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
		if runes[i] == '{' || runes[i] == '}' || runes[i] == '[' || runes[i] == ']' {
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

		// Literals: true, false, null, numbers
		start := i
		for i < n && !isJsonDelimiter(runes[i]) {
			i++
		}
		token := string(runes[start:i])

		var nodeName string
		switch {
		case token == "true" || token == "false" || token == "null":
			nodeName = "constant.builtin"
		case isJsonNumber(token):
			nodeName = "constant.numeric"
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

func isJsonDelimiter(r rune) bool {
	return r == ' ' || r == '\t' || r == ':' || r == ',' ||
		r == '{' || r == '}' || r == '[' || r == ']' || r == '"' || r == '/'
}

func isJsonNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	r := rune(s[0])
	return unicode.IsDigit(r) || (r == '-' && len(s) > 1 && unicode.IsDigit(rune(s[1])))
}
