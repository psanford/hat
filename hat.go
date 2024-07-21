package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/psanford/hat/ansiparser"
	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/terminal"
	"github.com/psanford/hat/vt100"
)

func main() {

	term := terminal.NewTerm(int(os.Stdin.Fd()))

	ed := newEditor(os.Stdin, os.Stdout, os.Stderr, term)

	ctx := context.Background()
	ed.run(ctx)
}

type editor struct {
	term  terminal.Terminal
	vt100 *vt100.VT100

	buf *gapbuffer.GapBuffer

	promptLine     int // 1 indexed
	editorRowCount int

	parser *ansiparser.Parser

	debugLog io.Writer

	debugEventCh chan struct{}

	in io.Reader
	// out *os.File or io.Writer
	// err io.Writer

	err error
}

func newEditor(in io.Reader, out io.Writer, err io.Writer, term terminal.Terminal) *editor {
	ed := &editor{
		in:    in,
		term:  term,
		vt100: vt100.New(term),
		buf:   gapbuffer.New(2),
	}

	return ed
}

func (ed *editor) run(ctx context.Context) {
	ed.term.EnableRawMode()
	defer ed.term.Restore()

	debug, _ := os.Create("/tmp/hat.debug.log")
	ed.debugLog = debug
	// ed.buf.Debug = debug

	fmt.Fprintf(debug, "hello\n")

	prevTermCols, prevTermRows := ed.term.Size()
	prevRow, _ := ed.vt100.CursorPos()
	ed.promptLine = prevRow - 1
	lastInsertNewline := false

	ed.editorRowCount = prevTermRows - ed.promptLine

	ed.parser = ansiparser.New(context.Background())
	ed.parser.SetLogger(func(s string, i ...interface{}) {
		fmt.Fprintf(ed.debugLog, s, i...)
		fmt.Fprintln(ed.debugLog)
	})
	events := ed.parser.EventChan()

	fmt.Fprintf(debug, "before main loop\n")

MAIN_LOOP:
	for {
		row, col := ed.vt100.CursorPos()
		bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)
		endBufPos := ed.buf.Size()

		termCols, termRows := ed.term.Size()
		if prevTermCols != termCols || prevTermRows != termRows {
			fmt.Fprintf(debug, "terminal resize! oldterm:<%d, %d> newterm:<%d, %d>\n", prevTermCols, prevTermRows, termCols, termRows)
			// XXXXXX
			// handle terminal resize here

			prevTermCols, prevTermRows = termCols, termRows
		}

		if lastInsertNewline {
			if row == prevRow { // we're at the bottom of the terminal
				if ed.promptLine > 1 {
					ed.promptLine--
					ed.editorRowCount++
					fmt.Fprintf(debug, "prompt move up 1 line to %d\n", ed.promptLine)
				}
			} else {
				fmt.Fprintf(debug, "prompt at top\n")
			}

			lastInsertNewline = false
		}

		if row <= ed.promptLine {
			fmt.Fprintf(debug, "we've fallen out of bounds! ed.promptLine=%d, row=%d\n", ed.promptLine, row)
			// panic(fmt.Sprintf("we've fallen out of bounds! ed.promptLine=%d, row=%d", ed.promptLine, row))
		}

		prevRow = row

		ed.writeDebugTerminalState()

		fmt.Fprintf(debug, "loop_start row: %d, col: %d, term:<%d, %d> editorRowCount=%d bufPos: %d, bufLine:%d\n", row, col, termCols, termRows, ed.editorRowCount, bufPos, ed.getLineNumber())

		ed.readBytes()

		for {
			var e ansiparser.Event
			select {
			case e = <-events:
				fmt.Fprintf(debug, "event!!!: %T %v\n", e, e)
				select {
				case ed.debugEventCh <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return
			default:
				fmt.Fprintf(debug, "CONTINUE MAIN_LOOP\n")

				continue MAIN_LOOP
			}

			fmt.Fprintf(debug, "event: %T %v\n", e, e)

			switch ee := e.(type) {
			case ansiparser.Character:
				c := ee.Char
				if c == 0x7F { // ASCII DEL (backspace)
					ed.deletePrevChar()
				} else if c == '\r' {
					fmt.Fprintf(debug, "loop: is newline\n")
					ed.buf.Insert([]byte{'\n'})
					ed.vt100.Write([]byte("\r\n"))
					lastInsertNewline = true
				} else if c == ctrlC || c == ctrlD {
					break MAIN_LOOP
				} else if c == ctrlA { // ctrl-a
					lineStart, _ := ed.buf.GetLine(0)
					ed.buf.Seek(int64(lineStart), io.SeekStart)
					ed.vt100.MoveTo(row, 1)
				} else if c == ctrlE { // ctrl-e
					lineStart, lineEnd := ed.buf.GetLine(0)

					if lineEnd == endBufPos-1 {
						lineEnd = endBufPos
					}

					ed.buf.Seek(int64(lineEnd), io.SeekStart)
					ed.vt100.MoveTo(row, 1+lineEnd-lineStart)
				} else if c == ctrlL {
					// redraw the section of the terminal we own
					ed.redrawVisible()

				} else {
					fmt.Fprintf(debug, "loop: is plain char\n")
					fmt.Fprintf(ed.debugLog, "write char %d %x %c\n", c, c, c)
					ed.buf.Insert([]byte{c})

					// goto beginning of row
					ed.vt100.MoveTo(row, 1)
					// clear line
					ed.vt100.ClearToEndOfLine()
					// rewrite line
					lineStart, lineEnd := ed.buf.GetLine(0)
					lineBuf := make([]byte, lineEnd-lineStart+1)
					ed.buf.ReadAt(lineBuf, int64(lineStart))
					ed.vt100.Write(lineBuf)

					// move cursor back to correct position
					colOffset := int(bufPos) + 1 - lineStart
					colOffset++ // inc b/c the terminal coords are 1 based
					ed.vt100.MoveTo(row, colOffset)
				}
			case ansiparser.DeleteCharater:
				// del
				ed.forwardChar()
				ed.deletePrevChar()
			case ansiparser.CursorMovement:
				switch ee.Direction {
				case ansiparser.Up:
					prevStart, prevEnd := ed.buf.GetLine(-1)
					if prevStart == -1 && prevEnd == -1 {
						// we're on the first line
						continue
					}
					curStart, _ := ed.buf.GetLine(0)
					if curStart == prevStart {
						continue
					}

					offsetCurLine := int(bufPos) - curStart

					fmt.Fprintf(debug, "moveup: pos: %d curStart: %d prevstart: %d prevEnd: %d\n", bufPos, curStart, prevStart, prevEnd)

					rowWidth := prevEnd - prevStart
					if offsetCurLine > rowWidth {
						offsetCurLine = rowWidth
					}

					ed.buf.Seek(int64(prevStart+offsetCurLine), io.SeekStart)
					ed.vt100.MoveTo(row-1, offsetCurLine+1)
				case ansiparser.Down:
					nextStart, nextEnd := ed.buf.GetLine(1)
					if nextStart == -1 {
						fmt.Fprintf(debug, "<down>: getLine(1) Failed, we think we're at the end of the buffer\n")
						continue
					}

					curStart, curEnd := ed.buf.GetLine(0)
					fmt.Fprintf(debug, "<down>: cur:%d curStart:%d curEnd:%d nextStart:%d nextEnd:%d\n", bufPos, curStart, curEnd, nextStart, nextEnd)

					offsetCurLine := int(bufPos) - curStart

					rowWidth := nextEnd - nextStart

					if offsetCurLine > rowWidth {
						offsetCurLine = rowWidth
					}

					ed.buf.Seek(int64(nextStart+offsetCurLine), io.SeekStart)
					ed.vt100.MoveTo(row+1, offsetCurLine+1)

				case ansiparser.Forward:
					ed.forwardChar()
				case ansiparser.Backward:
					startLine, _ := ed.buf.GetLine(0)
					if bufPos == 0 {
						// we're at the beginning of the buffer
					} else {
						if bufPos > int64(startLine) {
							fmt.Fprintf(ed.debugLog, "LEFT: just left 1 (to: %d, %d)\n", row, col-1)
							ed.buf.Seek(-1, io.SeekCurrent)
							ed.vt100.MoveTo(row, col-1)
						}
					}
				}
			default:
				fmt.Fprintf(debug, "unhandled event: %T %v\n", e, e)
			}

			info := ed.buf.DebugInfo()
			os.WriteFile("/tmp/hat.current.buffer", info.Bytes(), 0600)
		}
	}

	f, err := os.Create("/tmp/hat.out")
	if err != nil {
		panic(err)
	}
	ed.buf.Seek(0, io.SeekStart)
	_, err = io.Copy(f, ed.buf)
	if err != nil {
		panic(err)
	}
	f.Close()
}

func (ed *editor) writeDebugTerminalState() {
	termCols, termRows := ed.term.Size()
	curRow, curCol := ed.vt100.CursorPos()

	f, err := os.Create("/tmp/hat.current.terminal")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Fprintln(f, strings.Repeat("@", termCols))
	fmt.Fprintf(f, "@@ w:%d h:%d cur_row:%d cur_col:%d prompt_line:%d @@\n", termCols, termRows, curRow, curCol, ed.promptLine)

	for row := 1; row <= termRows; row++ {
		bufRowOffset := row - curRow
		startLine, endLine := ed.buf.GetLine(bufRowOffset)

		if startLine < 0 && endLine < 0 {
			fmt.Fprintln(f, strings.Repeat("@", termCols))
			continue
		}

		lineText := make([]byte, endLine-startLine+1)
		ed.buf.ReadAt(lineText, int64(startLine))

		for col := 1; col <= termCols; col++ {
			if row == curRow && col == curCol {
				f.Write([]byte("_"))
				continue
			}

			if col <= len(lineText) {
				b := lineText[col-1]
				if b == '\n' {
					f.Write([]byte{'$'})
				} else {
					f.Write([]byte{b})
				}
			} else {
				f.Write([]byte{'~'})
			}
		}
		f.Write([]byte{'\n'})
	}
}

// redrawVisible redraws the current editor viewport.
// The editor viewport is the number of lines we've used so far in the terminal.
func (ed *editor) redrawVisible() {
	cursorRow, _ := ed.vt100.CursorPos()
	_, termRows := ed.term.Size()

	ed.vt100.SaveCursorPos()

	editorStartRow := termRows + 1 - ed.editorRowCount
	fmt.Fprintf(ed.debugLog, "!!! redrawVisible: start_row=%d cursor_row=%d row_count=%d  term_rows=%d\n", editorStartRow, cursorRow, ed.editorRowCount, termRows)

	for row := editorStartRow; row < editorStartRow+ed.editorRowCount; row++ {
		offset := row - cursorRow
		ed.vt100.MoveTo(row, 1)
		ed.vt100.ClearToEndOfLine()
		bufStartLine, bufEndLine := ed.buf.GetLine(offset)
		fmt.Fprintf(ed.debugLog, "!!! redrawVisible: row=%d offset=%d bufstart=%d bufend=%d size=%d start=%d\n", row, offset, bufStartLine, bufEndLine, bufEndLine+1-bufStartLine, bufStartLine)
		fmt.Fprintf(ed.debugLog, "!!! info=%s\n", ed.buf.DebugInfo())
		if bufStartLine == -1 {
			fmt.Fprintf(ed.debugLog, "row=%d empty line\n", row)
			continue
		}

		line := make([]byte, bufEndLine+1-bufStartLine)
		ed.buf.ReadAt(line, int64(bufStartLine))
		fmt.Fprintf(ed.debugLog, "!!! redrawVisible: write_line=<%s>\n", line)
		ed.vt100.Write(line)
	}

	ed.vt100.RestoreCursorPos()
}

func (ed *editor) forwardChar() {
	bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)
	endBufPos := ed.buf.Size()
	row, col := ed.vt100.CursorPos()

	// if we're at the very end of the file our cursor should be at lastpos+1
	_, eolPos := ed.buf.GetLine(0)
	fmt.Fprintf(ed.debugLog, "right: bufPos=%d  eolPos=%d endBufPos=%d", bufPos, eolPos, endBufPos)
	if eolPos == endBufPos-1 {
		eolPos = endBufPos
	}
	if bufPos == int64(eolPos) {
		fmt.Fprintf(ed.debugLog, "RIGHT STOPPED: at end of buffer bufPos=%d\n", bufPos)
		return
	}
	newPos, _ := ed.buf.Seek(1, io.SeekCurrent)
	fmt.Fprintf(ed.debugLog, "request RIGHT: bufPos=%d, newPos=%d eolPos=%d endPos=%d\n", bufPos, newPos, eolPos, endBufPos)
	ed.vt100.MoveTo(row, col+1)
}

func (ed *editor) deletePrevChar() {
	bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)
	row, _ := ed.vt100.CursorPos()

	deleted := ed.buf.Delete(1)
	if len(deleted) > 0 && deleted[0] == '\n' {
		// we've deleted the previous newline. We need to redraw the previous lines and all following lines
		lineStart, _ := ed.buf.GetLine(0)
		lineOffset := bufPos - int64(lineStart)
		ed.vt100.MoveTo(row-1, int(lineOffset))
		ed.redrawVisible()
		return
	}
	// goto beginning of row
	ed.vt100.MoveTo(row, 1)
	// clear line
	ed.vt100.ClearToEndOfLine()
	// rewrite line
	lineStart, lineEnd := ed.buf.GetLine(0)
	lineBuf := make([]byte, lineEnd-lineStart+1)
	ed.buf.ReadAt(lineBuf, int64(lineStart))
	ed.vt100.Write(lineBuf)

	// move cursor back to correct position
	colOffset := int(bufPos) + -1 - lineStart
	colOffset++ // inc b/c the terminal coords are 1 based
	ed.vt100.MoveTo(row, colOffset)
}

func (ed *editor) cursorPos() (row, col int) {
	return ed.vt100.CursorPos()
}

func (ed *editor) readChar() (byte, error) {
	if ed.err != nil {
		return 0, ed.err
	}
	b := make([]byte, 1)
	_, err := ed.in.Read(b)
	if err != nil {
		ed.err = err
	}

	_, err = ed.parser.Write(b)
	if err != nil {
		panic(err)
	}

	return b[0], err
}

func (ed *editor) readBytes() (int, error) {
	if ed.err != nil {
		return 0, ed.err
	}
	b := make([]byte, 128)
	n, err := ed.in.Read(b)
	if err != nil {
		ed.err = err
	}

	fmt.Fprintf(ed.debugLog, "ed_read: %+v\n", b[:n])
	_, err = ed.parser.Write(b[:n])
	if err != nil {
		ed.err = err
	}

	return n, err
}

const (
	ctrlA = 0x01
	ctrlB = 0x02
	ctrlC = 0x03
	ctrlD = 0x04
	ctrlE = 0x05
	ctrlL = 0x0C
)

func ctrlKey(c byte) byte {
	return c & 0x1f
}

func bail(err error) {
	panic(err)
}

func (ed *editor) getLineNumber() int {
	startPos, _ := ed.buf.Seek(0, io.SeekCurrent)
	var count int
	prev := -1
	for {
		curStart, _ := ed.buf.GetLine(0)
		if curStart == 0 {
			break
		}
		if curStart == prev {
			fmt.Printf("%s\n", ed.buf.DebugInfo())
			panic("Failed to seek")
		}
		prev = curStart

		ed.buf.Seek(int64(curStart-1), io.SeekStart)
		count++
	}

	endPos, _ := ed.buf.Seek(startPos, io.SeekStart)
	if startPos != endPos {
		panic(fmt.Sprintf("getLineNumber: %d %d\n", startPos, endPos))
	}

	return count
}
