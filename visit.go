package wig

import "strings"

// VisitHandlerFunc handles visit-line behavior for special buffer types.
// Returns true if the handler handled the visit, false to fall through
// to the default flat-format behavior.
type VisitHandlerFunc func(ctx Context, sourceBuf *Buffer, movement func(Context)) bool

var visitHandlers []VisitHandlerFunc
var lastVisitSource *Buffer

// SetVisitSource stores the last used result buffer (e.g. [rg] or [rgcollect...])
// so that :cn and :cp can find it even if it's no longer in a window.
func SetVisitSource(buf *Buffer) {
	if buf != nil {
		lastVisitSource = buf
	}
}

// RegisterVisitHandler registers a handler for visit-line behavior.
// Called by packages that cannot be imported by wig (e.g. rgcollect)
// to handle special buffer types like [rg].
func RegisterVisitHandler(fn VisitHandlerFunc) {
	visitHandlers = append(visitHandlers, fn)
}

type VisitOptions struct {
	Movement      func(Context)
	Center        bool
	TargetWin     *Window
	ParseLocation bool
	Cursor        *Cursor
}

// findVisitSourceBuffer locates the buffer that CmdVisitNextLine / CmdVisitPrevLine
// should use as a source when jumping between entries. It mirrors the
// original rgcollect.visitLine design:
// 1. Search all windows for a buffer whose FilePath starts with "[rgcollect".
// 2. If not found, fallback to the "other" window (not active) if it exists.
// 3. If still not found, return nil.
func findVisitSourceBuffer(e *Editor) *Buffer {
	// 1. Prefer the last known source buffer if it still exists
	if lastVisitSource != nil {
		for _, buf := range e.Buffers {
			if buf == lastVisitSource {
				return buf
			}
		}
	}

	// 2. Search all windows for a result buffer ([rgcollect], [rg], or [quickfix]).
	for _, win := range e.Windows {
		if win.Buffer() != nil {
			fp := win.Buffer().FilePath
			if strings.HasPrefix(fp, "[rgcollect") || fp == "[rg]" || fp == "[quickfix]" {
				return win.Buffer()
			}
		}
	}

	// 3. Search all buffers (may not be visible after opening a file replaced it).
	for _, buf := range e.Buffers {
		if strings.HasPrefix(buf.FilePath, "[rgcollect") || buf.FilePath == "[rg]" || buf.FilePath == "[quickfix]" {
			return buf
		}
	}

	e.EchoMessage("rgcollect buffer not visible. using other window.")
	if len(e.Windows) > 1 {
		for _, win := range e.Windows {
			if win == e.ActiveWindow() {
				continue
			}
			return win.Buffer()
		}
	}

	return nil
}

func VisitAtLine(ctx Context, sourceBuf *Buffer, opts VisitOptions) error {
	var sourceWin *Window
	for _, win := range ctx.Editor.Windows {
		if win.Buffer() == sourceBuf {
			sourceWin = win
			break
		}
	}

	if sourceWin == nil {
		if ctx.Editor.ActiveWindow().Buffer() == sourceBuf {
			sourceWin = ctx.Editor.ActiveWindow()
		} else if strings.HasPrefix(sourceBuf.FilePath, "[rgcollect") || sourceBuf.FilePath == "[rg]" || sourceBuf.FilePath == "[quickfix]" {
			// If rg buffer is not in a window, use the active window to get/update its cursor.
			sourceWin = ctx.Editor.ActiveWindow()
		} else {
			ctx.Editor.EchoMessage("source buffer is not visible")
			return nil
		}
	}

	if opts.Movement != nil {
		nctx := ctx.Editor.NewContext()
		nctx.Buf = sourceBuf
		nctx.Win = sourceWin
		opts.Movement(nctx)
	}

	var targetCursor *Cursor
	if opts.Cursor != nil {
		targetCursor = opts.Cursor
	} else if opts.ParseLocation {
		bufCur := WindowCursorGet(sourceWin, sourceBuf)
		line := CursorLine(sourceBuf, bufCur)
		if line == nil {
			return nil
		}
		filename, lineNum, chNum := ParseFileLocation(line.Value.String(), 0)
		if filename == "" {
			ctx.Editor.EchoMessage("no file path found under cursor")
			return nil
		}
		var err error
		ctx.Buf, err = ctx.Editor.OpenFile(filename)
		if err != nil {
			return err
		}
		targetCursor = &Cursor{Line: lineNum - 1, Char: chNum}
	}

	targetWin := opts.TargetWin
	if targetWin == nil {
		if len(ctx.Editor.Windows) > 1 {
			curIdx := 0
			for i, w := range ctx.Editor.Windows {
				if w == sourceWin {
					curIdx = i
					break
				}
			}
			nextIdx := (curIdx + 1) % len(ctx.Editor.Windows)
			targetWin = ctx.Editor.Windows[nextIdx]
		} else {
			targetWin = ctx.Editor.ActiveWindow()
		}
	}

	ctx.Win = targetWin
	if targetCursor != nil {
		targetWin.VisitBuffer(ctx, *targetCursor)
	} else {
		targetWin.VisitBuffer(ctx)
	}

	if opts.Center {
		nctx := ctx.Editor.NewContext()
		nctx.Buf = ctx.Buf
		nctx.Win = targetWin
		CmdCursorCenter(nctx)
	}

	return nil
}

func CmdVisitNextLine(ctx Context) {
	sourceBuf := findVisitSourceBuffer(ctx.Editor)
	if sourceBuf == nil {
		return
	}

	for _, handler := range visitHandlers {
		if handler(ctx, sourceBuf, CmdCursorLineDown) {
			return
		}
	}

	VisitAtLine(ctx, sourceBuf, VisitOptions{
		Movement:      CmdCursorLineDown,
		ParseLocation: true,
		TargetWin:     ctx.Editor.ActiveWindow(),
	})
}

func CmdVisitPrevLine(ctx Context) {
	sourceBuf := findVisitSourceBuffer(ctx.Editor)
	if sourceBuf == nil {
		return
	}

	for _, handler := range visitHandlers {
		if handler(ctx, sourceBuf, CmdCursorLineUp) {
			return
		}
	}

	VisitAtLine(ctx, sourceBuf, VisitOptions{
		Movement:      CmdCursorLineUp,
		ParseLocation: true,
		TargetWin:     ctx.Editor.ActiveWindow(),
	})
}
