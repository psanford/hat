package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func main() {
	origTerm := enableRawMode()
	defer restoreTerminal(origTerm)

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
	}
}

func enableRawMode() (origState *unix.Termios) {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		panic(err)
	}

	orig := *termios

	// disable echo, canonical mode (read byte at a time instead of line)
	termios.Lflag &^= unix.ECHO | unix.ICANON
	// disable flowcontrol (ctrl-{s,q})
	termios.Iflag &^= unix.IXON
	// disable terminal translation of \r to \n (fix ctrl-m)
	termios.Iflag &^= unix.ICRNL
	// disable postprocessing (translation of \n to \r\n)
	termios.Oflag &^= unix.OPOST

	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		panic(err)
	}

	return &orig
}

const (
	vt100ClearAfterCursor  = "\x1b[0J"
	vt100ClearBeforeCursor = "\x1b[1J"
	vt100ClearEntireScreen = "\x1b[2J"
)

func restoreTerminal(termios *unix.Termios) {
	fd := int(os.Stdin.Fd())
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		panic(err)
	}
}

func ctrlKey(c byte) byte {
	return c & 0x1f
}

func bail(err error) {
	panic(err)
}
