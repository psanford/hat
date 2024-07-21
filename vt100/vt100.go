package vt100

import (
	"bytes"
	"fmt"

	"github.com/psanford/hat/terminal"
)

type VT100 struct {
	term terminal.Terminal
}

func New(t terminal.Terminal) *VT100 {
	return &VT100{
		term: t,
	}
}

func (t *VT100) Write(b []byte) (int, error) {
	return t.term.Write(b)
}

func (t *VT100) CursorPos() (row, col int) {
	if _, err := t.term.Write([]byte(vt100GetCursorActivePos)); err != nil {
		panic(err)
	}

	b, err := t.term.ReadControl()
	if err != nil {
		panic(err)
	}
	buf := bytes.NewBuffer(b)
	_, err = fmt.Fscanf(buf, "\x1b[%d;%dR", &row, &col)
	if err != nil {
		panic(err)
	}

	return
}

func (t *VT100) SaveCursorPos() {
	t.term.Write([]byte(vt100SaveCursorPosition))
}

func (t *VT100) RestoreCursorPos() {
	t.term.Write([]byte(vt100RestoreCursorPosition))
}

func (t *VT100) MoveTo(line, col int) {
	t.term.Write([]byte(fmt.Sprintf(vt100CursorPosition, line, col)))
}

func (t *VT100) ClearToEndOfLine() {
	t.term.Write([]byte(vt100ClearToEndOfLine))
}

const (
	// vt100ClearAfterCursor  = "\x1b[0J"
	// vt100ClearBeforeCursor = "\x1b[1J"
	// vt100ClearEntireScreen = "\x1b[2J"

	// vt100CursorUp    = "\x1b[A"
	// vt100CursorDown  = "\x1b[B"
	// vt100CursorRight = "\x1b[C"
	// vt100CursorLeft  = "\x1b[D"

	vt100ClearToEndOfLine = "\x1b[K"

	vt100GetCursorActivePos = "\x1b[6n" // device status report (arg=6)

	vt100CursorPosition = "\x1b[%d;%dH"

	vt100SaveCursorPosition    = "\x1b7"
	vt100RestoreCursorPosition = "\x1b8"

	// ctrlA = 0x01
	// ctrlB = 0x02
	// ctrlC = 0x03
	// ctrlD = 0x04
	// ctrlE = 0x05
	// ctrlL = 0x0C
)
