package terminal

import (
	"os"

	"golang.org/x/sys/unix"
)

type Terminal interface {
	// Set terminal to RawMode
	EnableRawMode()
	// Restore terminal to original settings
	Restore()
	// Size returns the terminal size in number of columns, rows
	Size() (int, int)
	Write([]byte) (int, error)
	Read([]byte) (int, error)
}

type Term struct {
	*os.File
	termios *unix.Termios
	orig    unix.Termios

	fd int
}

func NewTerm(fd int) *Term {
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		panic(err)
	}

	f := os.NewFile(uintptr(fd), "terminal")

	return &Term{
		termios: termios,
		orig:    *termios,
		fd:      fd,
		File:    f,
	}
}

func (t *Term) EnableRawMode() {
	// disable echo, canonical mode (read byte at a time instead of line)
	t.termios.Lflag &^= unix.ECHO | unix.ICANON
	// disable flowcontrol (ctrl-{s,q})
	t.termios.Iflag &^= unix.IXON
	// disable terminal translation of \r to \n (fix ctrl-m)
	t.termios.Iflag &^= unix.ICRNL
	// disable postprocessing (translation of \n to \r\n)
	t.termios.Oflag &^= unix.OPOST

	if err := unix.IoctlSetTermios(t.fd, ioctlWriteTermios, t.termios); err != nil {
		panic(err)
	}
}

func (t *Term) Restore() {
	if err := unix.IoctlSetTermios(t.fd, ioctlWriteTermios, &t.orig); err != nil {
		panic(err)
	}
}

func (t *Term) Size() (cols, rows int) {
	ws, err := unix.IoctlGetWinsize(t.fd, unix.TIOCGWINSZ)
	if err != nil {
		panic(err)
	}
	return int(ws.Col), int(ws.Row)
}

type MockTerm struct {
}
