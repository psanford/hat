package main

import (
	"fmt"
	"io"
	"os"

	"github.com/psanford/hat/gapbuffer"
	"golang.org/x/sys/unix"
)

type editor struct {
	termios *unix.Termios
	orig    unix.Termios

	buf *gapbuffer.GapBuffer

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

	debug, _ := os.Create("/tmp/hat.debug")

	// cols, rows := ed.termSize()
	// fmt.Printf("cols: %d rows:%d\r\n", cols, rows)
	// row, col := ed.cursorPos()
	// fmt.Printf("pos: col=%d row=%d\r\n", col, row)

	for {
		row, col := ed.cursorPos()
		bufPos, _ := ed.buf.Seek(0, io.SeekCurrent)

		fmt.Fprintf(debug, "row: %d, col: %d, bufPos: %d\n", row, col, bufPos)

		c, err := ed.readChar()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		if c == '\x1b' {
			// escape seq
			c1, _ := ed.readChar()
			c2, err := ed.readChar()
			if err != nil {
				panic(err)
			}

			if c1 == '[' {
				switch c2 {
				case 'A':
					// up
					prevStart, prevEnd := ed.buf.GetLine(-1)
					curStart, _ := ed.buf.GetLine(0)
					if curStart == prevStart {
						continue
					}

					offsetCurLine := int(bufPos) - curStart

					rowWidth := prevEnd - prevStart
					if col-1 > rowWidth {
						os.Stdout.Write([]byte(moveTo(rowWidth, col-1)))
						ed.buf.Seek(int64(prevEnd), io.SeekStart)
					} else {
						os.Stdout.Write([]byte(vt100CursorUp))
						ed.buf.Seek(int64(prevStart+offsetCurLine), io.SeekStart)
					}

				case 'B':
					// down
					nextStart, nextEnd := ed.buf.GetLine(1)
					fmt.Fprintf(debug, "<down>: nextStart:%d nextEnd:%d\n", nextStart, nextEnd)

					curStart, _ := ed.buf.GetLine(0)
					if nextStart == curStart {
						continue
					}

					offsetCurLine := int(bufPos) - curStart

					rowWidth := nextEnd - nextStart
					if col-1 > rowWidth {
						os.Stdout.Write([]byte(moveTo(rowWidth, col-1)))
						ed.buf.Seek(int64(nextEnd), io.SeekStart)
					} else {
						os.Stdout.Write([]byte(vt100CursorDown))
						ed.buf.Seek(int64(nextStart+offsetCurLine), io.SeekStart)
					}
				case 'C':
					// right
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
				case 'D':
					// left
					if col > 1 {
						ed.buf.Seek(-1, io.SeekCurrent)
						os.Stdout.Write([]byte(vt100CursorLeft))
					}
				}
			}
		} else if c == 0x7F {
			// ASCII DEL (backspace)
			ed.buf.Delete(1)

			// goto beginning of row
			os.Stdout.Write([]byte(moveTo(row, 1)))
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
			os.Stdout.Write([]byte(moveTo(row, colOffset)))
		} else if c == '\r' {
			ed.buf.Insert([]byte{'\n'})
			if _, err := os.Stdout.Write([]byte("\r\n")); err != nil {
				panic(err)
			}
		} else {
			ed.buf.Insert([]byte{c})

			// goto beginning of row
			os.Stdout.Write([]byte(moveTo(row, 1)))
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
			os.Stdout.Write([]byte(moveTo(row, colOffset)))
		}

		if c == ctrlKey('q') || c == ctrlKey('c') {
			break
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
	fmt.Printf("wrote /tmp/hat.out\n")
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
	return b[0], err
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
)

func (ed *editor) restoreTerminal() {
	if err := unix.IoctlSetTermios(ed.fd, ioctlWriteTermios, &ed.orig); err != nil {
		panic(err)
	}
}

func moveTo(line, col int) string {
	return fmt.Sprintf(vt100CursorPosition, line, col)
}

func ctrlKey(c byte) byte {
	return c & 0x1f
}

func bail(err error) {
	panic(err)
}
