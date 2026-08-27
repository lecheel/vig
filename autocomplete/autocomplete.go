package autocomplete

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func Register(e *wig.Editor) wig.AutocompleteFn {
	return func(ctx wig.Context) bool {
		if ctx.Editor.Snippets.Complete(ctx) {
			return true
		}
		cur := wig.ContextCursorGet(ctx)
		line := wig.CursorLine(ctx.Buf, cur)
		if line != nil {
			lineRunes := line.Value
			charIdx := cur.Char
			start := charIdx
			for start > 0 {
				r := lineRunes[start-1]
				if r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
					start--
				} else {
					break
				}
			}
			prefix := string(lineRunes[start:charIdx])
			if strings.HasPrefix(prefix, "./") || strings.HasPrefix(prefix, "../") {
				PathComplete(ctx)
				return true
			}
		}
		refreshFn := func() wig.CompletionItems {
			return ctx.Editor.Lsp.Completion(ctx.Buf)
		}
		ui.AutocompleteInit(
			ctx,
			wig.Position{
				Line: cur.Line,
				Char: cur.Char,
			},
			refreshFn(),
			refreshFn,
		)
		return true
	}
}

// getBufferCompletions scans the current buffer for words matching the 3+ char prefix
// under the cursor.
func getBufferCompletions(ctx wig.Context) wig.CompletionItems {
	cur := wig.ContextCursorGet(ctx)
	line := wig.CursorLine(ctx.Buf, cur)
	if line == nil {
		return wig.CompletionItems{}
	}

	lineRunes := line.Value
	charIdx := cur.Char
	if charIdx >= len(lineRunes) {
		charIdx = len(lineRunes) - 1
	}

	start := charIdx
	for start > 0 {
		r := lineRunes[start-1]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			start--
		} else {
			break
		}
	}

	prefix := string(lineRunes[start:charIdx])
	items := wig.CompletionItems{}

	if len(prefix) < 3 {
		return items
	}

	seen := make(map[string]bool)
	currentLine := ctx.Buf.Lines.First()
	for currentLine != nil {
		text := string(currentLine.Value)
		wordStart := -1
		for i, r := range text {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				if wordStart == -1 {
					wordStart = i
				}
			} else {
				if wordStart != -1 {
					word := text[wordStart:i]
					if len(word) >= 3 && strings.HasPrefix(strings.ToLower(word), strings.ToLower(prefix)) && word != prefix {
						if !seen[word] {
							seen[word] = true
							te := &wig.CompletionTextEdit{
								NewText: word,
								Insert: struct {
									Start struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									} `json:"start"`
									End struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									} `json:"end"`
								}{
									Start: struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									}{Line: cur.Line, Character: start},
									End: struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									}{Line: cur.Line, Character: charIdx},
								},
								Replace: struct {
									Start struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									} `json:"start"`
									End struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									} `json:"end"`
								}{
									Start: struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									}{Line: cur.Line, Character: start},
									End: struct {
										Line      int `json:"line"`
										Character int `json:"character"`
									}{Line: cur.Line, Character: charIdx},
								},
							}
							items.AddItem(word, word, te)
						}
					}
					wordStart = -1
				}
			}
		}
		currentLine = currentLine.Next()
	}

	return items
}

// wordlistCache holds the cached contents of ~/.config/wig/wordlist.txt.
var wordlistCache []string
var wordlistLoaded bool

// loadWordlist reads ~/.config/wig/wordlist.txt once and caches the result.
func loadWordlist() []string {
	if wordlistLoaded {
		return wordlistCache
	}
	wordlistLoaded = true

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".config", "wig", "wordlist.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		word := strings.TrimSpace(line)
		if word != "" {
			wordlistCache = append(wordlistCache, word)
		}
	}
	return wordlistCache
}

// makeCompletionTextEdit builds a CompletionTextEdit that replaces the
// range [start, end) on the given line with newText.
func makeCompletionTextEdit(newText string, line, start, end int) *wig.CompletionTextEdit {
	return &wig.CompletionTextEdit{
		NewText: newText,
		Insert: struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
			End struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"end"`
		}{
			Start: struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			}{Line: line, Character: start},
			End: struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			}{Line: line, Character: end},
		},
		Replace: struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
			End struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"end"`
		}{
			Start: struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			}{Line: line, Character: start},
			End: struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			}{Line: line, Character: end},
		},
	}
}

// getWordlistCompletions returns completion items from ~/.config/wig/wordlist.txt
// matching the prefix under the cursor.
func getWordlistCompletions(ctx wig.Context) wig.CompletionItems {
	cur := wig.ContextCursorGet(ctx)
	line := wig.CursorLine(ctx.Buf, cur)
	if line == nil {
		return wig.CompletionItems{}
	}

	lineRunes := line.Value
	charIdx := cur.Char
	if charIdx >= len(lineRunes) {
		charIdx = len(lineRunes) - 1
	}

	start := charIdx
	for start > 0 {
		r := lineRunes[start-1]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			start--
		} else {
			break
		}
	}

	prefix := string(lineRunes[start:charIdx])
	items := wig.CompletionItems{}

	if len(prefix) < 1 {
		return items
	}

	words := loadWordlist()
	lowerPrefix := strings.ToLower(prefix)
	seen := make(map[string]bool)
	for _, word := range words {
		if strings.HasPrefix(strings.ToLower(word), lowerPrefix) && word != prefix {
			if !seen[word] {
				seen[word] = true
				te := makeCompletionTextEdit(word, cur.Line, start, charIdx)
				items.AddItem(word, word, te)
			}
		}
	}

	return items
}

// LocalComplete triggers manual completion using both wordlist.txt and local buffer words.
// Words from wordlist.txt are shown first, followed by matches from the current buffer.
func LocalComplete(ctx wig.Context) {
	refreshFn := func() wig.CompletionItems {
		wlItems := getWordlistCompletions(ctx)
		bufItems := getBufferCompletions(ctx)

		seen := make(map[string]bool)
		items := wig.CompletionItems{}
		// Add wordlist completions first
		for _, item := range wlItems.Items {
			if !seen[item.Label] {
				seen[item.Label] = true
				items.Items = append(items.Items, item)
			}
		}
		// Add buffer completions second
		for _, item := range bufItems.Items {
			if !seen[item.Label] {
				seen[item.Label] = true
				items.Items = append(items.Items, item)
			}
		}
		return items
	}

	items := refreshFn()
	if len(items.Items) == 0 {
		ctx.Editor.EchoMessage("No local completions found")
		return
	}

	// If there is only one candidate, auto-complete it immediately without showing the popup.
	if len(items.Items) == 1 {
		item := items.Items[0]
		cur := wig.ContextCursorGet(ctx)
		line := wig.CursorLine(ctx.Buf, cur)

		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}

		wig.TextDelete(ctx.Buf, &wig.Selection{
			Start: wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.Start.Character},
			End:   wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.End.Character},
		})

		wig.TextInsert(ctx.Buf, line, item.TextEdit.Replace.Start.Character, item.TextEdit.NewText)
		cur.Char = item.TextEdit.Replace.Start.Character + utf8.RuneCountInString(item.TextEdit.NewText)
		return
	}

	cur := wig.ContextCursorGet(ctx)
	ui.AutocompleteInit(
		ctx,
		wig.Position{
			Line: cur.Line,
			Char: cur.Char,
		},
		items,
		refreshFn,
	)
}

// WordlistComplete triggers manual completion from ~/.config/wig/wordlist.txt.
func WordlistComplete(ctx wig.Context) {
	refreshFn := func() wig.CompletionItems {
		return getWordlistCompletions(ctx)
	}
	items := refreshFn()
	if len(items.Items) == 0 {
		ctx.Editor.EchoMessage("No wordlist completions found")
		return
	}
	if len(items.Items) == 1 {
		item := items.Items[0]
		cur := wig.ContextCursorGet(ctx)
		line := wig.CursorLine(ctx.Buf, cur)
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		wig.TextDelete(ctx.Buf, &wig.Selection{
			Start: wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.Start.Character},
			End:   wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.End.Character},
		})
		wig.TextInsert(ctx.Buf, line, item.TextEdit.Replace.Start.Character, item.TextEdit.NewText)
		cur.Char = item.TextEdit.Replace.Start.Character + utf8.RuneCountInString(item.TextEdit.NewText)
		return
	}
	cur := wig.ContextCursorGet(ctx)
	ui.AutocompleteInit(
		ctx,
		wig.Position{
			Line: cur.Line,
			Char: cur.Char,
		},
		items,
		refreshFn,
	)
}

func PathComplete(ctx wig.Context) {
	cur := wig.ContextCursorGet(ctx)
	line := wig.CursorLine(ctx.Buf, cur)
	if line == nil {
		return
	}
	lineRunes := line.Value
	charIdx := cur.Char

	start := charIdx
	for start > 0 {
		r := lineRunes[start-1]
		if r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
			start--
		} else {
			break
		}
	}
	prefix := string(lineRunes[start:charIdx])

	if !strings.HasPrefix(prefix, "./") && !strings.HasPrefix(prefix, "../") {
		LocalComplete(ctx)
		return
	}

	refreshFn := func() wig.CompletionItems {
		return getPathCompletions(ctx, prefix, start, charIdx)
	}

	items := refreshFn()
	if len(items.Items) == 0 {
		ctx.Editor.EchoMessage("No path completions found")
		return
	}

	if len(items.Items) == 1 {
		item := items.Items[0]
		line := wig.CursorLine(ctx.Buf, cur)
		if ctx.Buf.TxStart() {
			defer ctx.Buf.TxEnd()
		}
		wig.TextDelete(ctx.Buf, &wig.Selection{
			Start: wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.Start.Character},
			End:   wig.Cursor{Line: cur.Line, Char: item.TextEdit.Replace.End.Character},
		})
		wig.TextInsert(ctx.Buf, line, item.TextEdit.Replace.Start.Character, item.TextEdit.NewText)
		cur.Char = item.TextEdit.Replace.Start.Character + utf8.RuneCountInString(item.TextEdit.NewText)
		return
	}

	ui.AutocompleteInit(
		ctx,
		wig.Position{
			Line: cur.Line,
			Char: cur.Char,
		},
		items,
		refreshFn,
	)
}

func getPathCompletions(ctx wig.Context, prefix string, start, end int) wig.CompletionItems {
	cur := wig.ContextCursorGet(ctx)
	items := wig.CompletionItems{}

	baseDir := "."
	filter := ""
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash != -1 {
		baseDir = prefix[:lastSlash+1]
		filter = prefix[lastSlash+1:]
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return items
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(filter, ".") {
			continue
		}
		if strings.HasPrefix(name, filter) {
			fullName := baseDir + name
			if entry.IsDir() {
				fullName += "/"
			}
			te := makeCompletionTextEdit(fullName, cur.Line, start, end)
			items.AddItem(fullName, fullName, te)
		}
	}
	return items
}
