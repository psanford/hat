package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/psanford/hat/ansiparser"
	"github.com/psanford/hat/gapbuffer"
	"golang.org/x/sys/unix"
)

type editor struct {
	termios *unix.Termios
	orig    unix.Termios

	buf *gapbuffer.GapBuffer

	promptLine     int
	editorRowCount int

	parser *ansiparser.Parser

	debug              io.Writer
	debugCurrentBuffer *os.File

	in  *os.File
	out *os.File

	fd  int
	err error
}

func main() {
	ed := &editor{
		in:  os.Stdin,
		out: os.Stdout,
		fd:  int(os.Stdin.Fd()),
		buf: gapbuffer.New(2),
	}

	termios, err := unix.IoctlGetTermios(ed.fd, ioctlReadTermios)
	if err != nil {
		panic(err)
	}

	ed.termios = termios
	ed.orig = *termios

	ed.enableRawMode()
	defer ed.restoreTerminal()

	debug, _ := os.Create("/tmp/hat.debug.log")
	ed.debug = debug
	// ed.buf.Debug = debug

	prevTermCols, prevTermRows := ed.termSize()
	prevRow, _ := ed.cursorPos()
	ed.promptLine = prevRow - 1
	lastInsertNewline := false

	ed.editorRowCount = prevTermRows - ed.promptLine

	ed.parser = ansiparser.New(context.Background())
	events := ed.parser.EventChan()

MAIN_LOOP:
	for {
		row, col := ed.cursorPos()
		bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)
		endBufPos := ed.buf.Size()

		termCols, termRows := ed.termSize()
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
			fmt.Fprintf(debug, "we've fallen out of bounds! ed.promptLine=%d, row=%d", ed.promptLine, row)
			// panic(fmt.Sprintf("we've fallen out of bounds! ed.promptLine=%d, row=%d", ed.promptLine, row))
		}

		prevRow = row

		fmt.Fprintf(debug, "loop_start row: %d, col: %d, term:<%d, %d> editorRowCount=%d bufPos: %d, bufLine:%d\n", row, col, termCols, termRows, ed.editorRowCount, bufPos, ed.getLineNumber())

		ed.readBytes()

		for {
			var e ansiparser.Event
			select {
			case e = <-events:
			default:
				continue MAIN_LOOP
			}

			fmt.Fprintf(debug, "event: %T %v\n", e, e)

			switch ee := e.(type) {
			case ansiparser.Character:
				c := ee.Char
				if c == 0x7F { // ASCII DEL (backspace)
					deleted := ed.buf.Delete(1)
					if len(deleted) > 0 && deleted[0] == '\n' {
						// we've deleted the previous newline. We need to redraw the previous lines and all following lines
						lineStart, _ := ed.buf.GetLine(0)
						lineOffset := bufPos - int64(lineStart)
						ed.out.Write(moveTo(row-1, int(lineOffset)))
						ed.redrawVisible()
						continue
					}
					// goto beginning of row
					ed.in.Write(moveTo(row, 1))
					// clear line
					ed.in.Write([]byte(vt100ClearToEndOfLine))
					// rewrite line
					lineStart, lineEnd := ed.buf.GetLine(0)
					lineBuf := make([]byte, lineEnd-lineStart+1)
					ed.buf.ReadAt(lineBuf, int64(lineStart))
					ed.in.Write(lineBuf)
					// move cursor back to correct position
					colOffset := int(bufPos) + -1 - lineStart
					colOffset++ // inc b/c the terminal coords are 1 based
					ed.in.Write(moveTo(row, colOffset))
				} else if c == '\r' {
					fmt.Fprintf(debug, "loop: is newline\n")
					ed.buf.Insert([]byte{'\n'})
					ed.in.Write([]byte("\r\n"))
					lastInsertNewline = true
				} else if c == ctrlC || c == ctrlD {
					break MAIN_LOOP
				} else if c == ctrlA { // ctrl-a
					lineStart, _ := ed.buf.GetLine(0)
					ed.buf.Seek(int64(lineStart), io.SeekStart)
					ed.out.Write(moveTo(row, 1))
				} else if c == ctrlE { // ctrl-e
					lineStart, lineEnd := ed.buf.GetLine(0)

					if lineEnd == endBufPos-1 {
						lineEnd = endBufPos
					}

					ed.buf.Seek(int64(lineEnd), io.SeekStart)
					ed.out.Write(moveTo(row, 1+lineEnd-lineStart))
				} else if c == ctrlL {
					// redraw the section of the terminal we own
					ed.redrawVisible()

				} else {
					fmt.Fprintf(debug, "loop: is plain char\n")
					fmt.Fprintf(ed.debug, "write char %d %x %c\n", c, c, c)
					ed.buf.Insert([]byte{c})

					// goto beginning of row
					ed.in.Write(moveTo(row, 1))
					// clear line
					ed.in.Write([]byte(vt100ClearToEndOfLine))
					// rewrite line
					lineStart, lineEnd := ed.buf.GetLine(0)
					lineBuf := make([]byte, lineEnd-lineStart+1)
					ed.buf.ReadAt(lineBuf, int64(lineStart))
					ed.in.Write(lineBuf)
					// move cursor back to correct position
					colOffset := int(bufPos) + 1 - lineStart
					colOffset++ // inc b/c the terminal coords are 1 based
					ed.in.Write(moveTo(row, colOffset))
				}
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
					ed.in.Write(moveTo(row-1, offsetCurLine+1))
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
					ed.in.Write(moveTo(row+1, offsetCurLine+1))

				case ansiparser.Forward:
					// if we're at the very end of the file our cursor should be at lastpos+1
					_, eolPos := ed.buf.GetLine(0)
					fmt.Fprintf(ed.debug, "right: bufPos=%d  eolPos=%d endBufPos=%d", bufPos, eolPos, endBufPos)
					if eolPos == endBufPos-1 {
						eolPos = endBufPos
					}
					if bufPos == int64(eolPos) {
						fmt.Fprintf(ed.debug, "RIGHT STOPPED: at end of buffer bufPos=%d\n", bufPos)
						continue
					}
					newPos, _ := ed.buf.Seek(1, io.SeekCurrent)
					fmt.Fprintf(ed.debug, "request RIGHT: bufPos=%d, newPos=%d eolPos=%d endPos=%d\n", bufPos, newPos, eolPos, endBufPos)
					ed.in.Write(moveTo(row, col+1))
				case ansiparser.Backward:
					startLine, _ := ed.buf.GetLine(0)
					if bufPos == 0 {
						// we're at the beginning of the buffer
					} else {
						if bufPos > int64(startLine) {
							fmt.Fprintf(ed.debug, "LEFT: just left 1 (to: %d, %d)\n", row, col-1)
							ed.buf.Seek(-1, io.SeekCurrent)
							ed.in.Write(moveTo(row, col-1))
						}
					}
				}
			default:
				fmt.Fprintf(debug, "unhandled event: %T %v\n", e, e)
			}

			info := ed.buf.DebugInfo()
			debufBuf, _ := os.Create("/tmp/hat.current.buffer")
			ed.debugCurrentBuffer = debufBuf

			ioutil.WriteFile("/tmp/hat.current.buffer", info.Bytes(), 0600)
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

// redrawVisible redraws the current editor viewport.
// The editor viewport is the number of lines we've used so far in the terminal.
func (ed *editor) redrawVisible() {
	cursorRow, _ := ed.cursorPos()
	_, termRows := ed.termSize()

	ed.out.Write([]byte(vt100SaveCursorPosition))

	editorStartRow := termRows + 1 - ed.editorRowCount
	fmt.Fprintf(ed.debug, "!!! redrawVisible: start_row=%d cursor_row=%d row_count=%d  term_rows=%d\n", editorStartRow, cursorRow, ed.editorRowCount, termRows)

	for row := editorStartRow; row < editorStartRow+ed.editorRowCount; row++ {
		offset := row - cursorRow
		ed.out.Write(moveTo(row, 1))
		ed.out.Write([]byte(vt100ClearToEndOfLine))
		bufStartLine, bufEndLine := ed.buf.GetLine(offset)
		fmt.Fprintf(ed.debug, "!!! redrawVisible: row=%d offset=%d bufstart=%d bufend=%d size=%d start=%d\n", row, offset, bufStartLine, bufEndLine, bufEndLine+1-bufStartLine, bufStartLine)
		fmt.Fprintf(ed.debug, "!!! info=%s\n", ed.buf.DebugInfo())
		if bufStartLine == -1 {
			fmt.Fprintf(ed.debug, "row=%d empty line\n", row)
			continue
		}

		line := make([]byte, bufEndLine+1-bufStartLine)
		ed.buf.ReadAt(line, int64(bufStartLine))
		fmt.Fprintf(ed.debug, "!!! redrawVisible: write_line=<%s>\n", line)
		ed.out.Write(line)
	}

	ed.out.Write([]byte(vt100RestoreCursorPosition))
}

func (ed *editor) cursorPos() (row, col int) {
	if _, err := os.Stdin.Write([]byte(vt100GetCursorActivePos)); err != nil {
		panic(err)
	}

	_, err := fmt.Fscanf(os.Stdin, "\x1b[%d;%dR", &row, &col)
	if err != nil {
		panic(err)
	}

	return
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

	_, err = ed.parser.Write(b[:n])
	if err != nil {
		ed.err = err
	}

	return n, err
}

func (ed *editor) termSize() (cols, rows int) {
	ws, err := unix.IoctlGetWinsize(ed.fd, unix.TIOCGWINSZ)
	if err != nil {
		panic(err)
	}
	return int(ws.Col), int(ws.Row)
}

func (ed *editor) enableRawMode() {
	// disable echo, canonical mode (read byte at a time instead of line)
	ed.termios.Lflag &^= unix.ECHO | unix.ICANON
	// disable flowcontrol (ctrl-{s,q})
	ed.termios.Iflag &^= unix.IXON
	// disable terminal translation of \r to \n (fix ctrl-m)
	ed.termios.Iflag &^= unix.ICRNL
	// disable postprocessing (translation of \n to \r\n)
	ed.termios.Oflag &^= unix.OPOST

	if err := unix.IoctlSetTermios(ed.fd, ioctlWriteTermios, ed.termios); err != nil {
		panic(err)
	}
}

const (
	vt100ClearAfterCursor  = "\x1b[0J"
	vt100ClearBeforeCursor = "\x1b[1J"
	vt100ClearEntireScreen = "\x1b[2J"

	vt100CursorUp    = "\x1b[A"
	vt100CursorDown  = "\x1b[B"
	vt100CursorRight = "\x1b[C"
	vt100CursorLeft  = "\x1b[D"

	vt100ClearToEndOfLine = "\x1b[K"

	vt100GetCursorActivePos = "\x1b[6n" // device status report (arg=6)

	vt100CursorPosition = "\x1b[%d;%dH"

	vt100SaveCursorPosition    = "\x1b7"
	vt100RestoreCursorPosition = "\x1b8"

	ctrlA = 0x01
	ctrlB = 0x02
	ctrlC = 0x03
	ctrlD = 0x04
	ctrlE = 0x05
	ctrlL = 0x0C
)

func (ed *editor) restoreTerminal() {
	if err := unix.IoctlSetTermios(ed.fd, ioctlWriteTermios, &ed.orig); err != nil {
		panic(err)
	}
}

func moveTo(line, col int) []byte {
	return []byte(fmt.Sprintf(vt100CursorPosition, line, col))
}

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
