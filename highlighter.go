package wig

import "github.com/gdamore/tcell/v2"

// Span represents a styled column interval within a single line.
type Span struct {
	StartCol uint16
	EndCol   uint16
	Style    tcell.Style
}

type Highlighter interface {
	// Full document build/re-build/init
	Build()
	TextChanged(EventTextChange)

	// Query syntax highlight spans for a specific line
	HighlightLine(lineNum int) []Span
}
