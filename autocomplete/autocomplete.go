package autocomplete

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

func Register(e *wig.Editor) wig.AutocompleteFn {
	return func(ctx wig.Context) bool {
		// Check for snippets first
		if ctx.Editor.Snippets.Complete(ctx) {
			return true
		}

		cur := wig.ContextCursorGet(ctx)
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

// BufferComplete triggers manual completion for local buffer words.
func BufferComplete(ctx wig.Context) {
	refreshFn := func() wig.CompletionItems {
		return getBufferCompletions(ctx)
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
