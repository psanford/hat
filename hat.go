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

	parser *ansiparser.Parser

	debug              io.Writer
	debugCurrentBuffer *os.File

	f   *os.File
	fd  int
	err error
}

func main() {
	ed := &editor{
		f:   os.Stdin,
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
	promptLine := prevRow - 1
	lastInsertNewline := false

	ed.parser = ansiparser.New(context.Background())
	events := ed.parser.EventChan()

MAIN_LOOP:
	for {
		row, col := ed.cursorPos()
		bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)

		termCols, termRows := ed.termSize()
		if prevTermCols != termCols || prevTermRows != termRows {
			fmt.Fprintf(debug, "terminal resize! oldterm:<%d, %d> newterm:<%d, %d>\n", prevTermCols, prevTermRows, termCols, termRows)
			// XXXXXX
			// handle terminal resize here

			prevTermCols, prevTermRows = termCols, termRows
		}

		if lastInsertNewline {
			if row == prevRow { // we're at the bottom of the terminal
				if promptLine > 1 {
					promptLine--
					fmt.Fprintf(debug, "prompt move up 1 line to %d\n", promptLine)
				}
			} else {
				fmt.Fprintf(debug, "prompt at top\n")
			}

			lastInsertNewline = false
		}

		prevRow = row

		fmt.Fprintf(debug, "loop_start row: %d, col: %d, term:<%d, %d> bufPos: %d, bufLine:%d\n", row, col, termCols, termRows, bufPos, ed.getLineNumber())

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
					}
					// goto beginning of row
					os.Stdout.Write(moveTo(row, 1))
					// clear line
					os.Stdout.Write([]byte(vt100ClearToEndOfLine))
					// rewrite line
					lineStart, lineEnd := ed.buf.GetLine(0)
					lineBuf := make([]byte, lineEnd-lineStart+1)
					ed.buf.ReadAt(lineBuf, int64(lineStart))
					os.Stdout.Write(lineBuf)
					// move cursor back to correct position
					colOffset := int(bufPos) + -1 - lineStart
					colOffset++ // inc b/c the terminal coords are 1 based
					os.Stdout.Write(moveTo(row, colOffset))
				} else if c == '\r' {
					fmt.Fprintf(debug, "loop: is newline\n")
					ed.buf.Insert([]byte{'\n'})
					os.Stdout.Write([]byte("\r\n"))
					lastInsertNewline = true
				} else if c == ctrlC || c == ctrlD {
					break MAIN_LOOP
				} else if c == ctrlA { // ctrl-a
				} else if c == ctrlE { // ctrl-e
				} else {
					fmt.Fprintf(debug, "loop: is plain char\n")
					fmt.Fprintf(ed.debug, "write char %d %x %c\n", c, c, c)
					ed.buf.Insert([]byte{c})

					// goto beginning of row
					os.Stdout.Write(moveTo(row, 1))
					// clear line
					os.Stdout.Write([]byte(vt100ClearToEndOfLine))
					// rewrite line
					lineStart, lineEnd := ed.buf.GetLine(0)
					lineBuf := make([]byte, lineEnd-lineStart+1)
					ed.buf.ReadAt(lineBuf, int64(lineStart))
					os.Stdout.Write(lineBuf)
					// move cursor back to correct position
					colOffset := int(bufPos) + 1 - lineStart
					colOffset++ // inc b/c the terminal coords are 1 based
					os.Stdout.Write(moveTo(row, colOffset))
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
					os.Stdout.Write(moveTo(row-1, offsetCurLine+1))
				case ansiparser.Down:
					nextStart, nextEnd := ed.buf.GetLine(1)
					if nextStart == -1 {
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
					os.Stdout.Write(moveTo(row+1, offsetCurLine+1))

				case ansiparser.Forward:
					_, endPos := ed.buf.GetLine(0)
					newPos, _ := ed.buf.Seek(1, io.SeekCurrent)
					if newPos == bufPos {
						// we're at the end of the buffer
					} else {
						if newPos <= int64(endPos) {
							os.Stdout.Write([]byte(vt100CursorRight))
						} else {
							os.Stdout.Write([]byte(vt100CursorDown))
							os.Stdout.Write([]byte{'\r'})
						}
					}
				case ansiparser.Backward:
					startLine, _ := ed.buf.GetLine(0)
					if bufPos == 0 {
						// we're at the beginning of the buffer
					} else {
						newPos, _ := ed.buf.Seek(-1, io.SeekCurrent)
						if newPos < int64(startLine) {
							newStartLine, newEndLine := ed.buf.GetLine(0)
							fmt.Fprintf(ed.debug, "LEFT: go up 1 line: row=%d width: %d-%d+1\n", row-1, newEndLine, newStartLine)
							fmt.Fprintf(ed.debug, "LEFT: go up (to: %d, %d)\n", row-1, newEndLine-newStartLine+1)
							os.Stdout.Write(moveTo(row-1, newEndLine-newStartLine+1))
						} else {
							fmt.Fprintf(ed.debug, "LEFT: just left 1 (to: %d, %d)\n", row, col-1)
							os.Stdout.Write(moveTo(row, col-1))
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
	_, err := ed.f.Read(b)
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
	n, err := ed.f.Read(b)
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

	ctrlA = 0x01
	ctrlB = 0x02
	ctrlC = 0x03
	ctrlD = 0x04
	ctrlE = 0x05
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
