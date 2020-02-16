package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

type editor struct {
	termios *unix.Termios
	orig    unix.Termios
	fd      int
}

func main() {
	ed := new(editor)

	ed.fd = int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(ed.fd, ioctlReadTermios)
	if err != nil {
		panic(err)
	}

	ed.termios = termios
	ed.orig = *termios

	ed.enableRawMode()
	defer ed.restoreTerminal()

	cols, rows := ed.termSize()

	fmt.Printf("cols: %d rows:%d\r\n", cols, rows)

	col, row := ed.cursorPos()
	fmt.Printf("pos: col=%d row=%d\r\n", col, row)

	for {
		b := make([]byte, 1)
		_, err := os.Stdin.Read(b)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		fmt.Printf("got: <%c> <%d>\r\n", b[0], b[0])

		if b[0] == ctrlKey('q') {
			break
		}
		col, row := ed.cursorPos()
		fmt.Printf("pos: col=%d row=%d\r\n", col, row)
	}
}

func (ed *editor) cursorPos() (col, row int) {
	if _, err := os.Stdin.Write([]byte(vt100GetCursorActivePos)); err != nil {
		panic(err)
	}

	_, err := fmt.Fscanf(os.Stdin, "\x1b[%d;%dR", &col, &row)
	if err != nil {
		panic(err)
	}

	return
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

	vt100GetCursorActivePos = "\x1b[6n" // device status report (arg=6)
)

func (ed *editor) restoreTerminal() {
	if err := unix.IoctlSetTermios(ed.fd, ioctlWriteTermios, &ed.orig); err != nil {
		panic(err)
	}
}

func ctrlKey(c byte) byte {
	return c & 0x1f
}

func bail(err error) {
	panic(err)
}
