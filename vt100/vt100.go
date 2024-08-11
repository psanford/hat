package vt100

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

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

// Size returns the terminal size in number of columns, rows
func (t *VT100) Size() TermCoord {

	cols, rows := t.term.Size()
	return TermCoord{
		Col: cols,
		Row: rows,
	}

}

func (t *VT100) Write(b []byte) (int, error) {
	return t.term.Write(b)
}

// A terminal location in terminal space (1 based)
type TermCoord struct {
	Row, Col int
}

func (t *VT100) CursorPos() (*TermCoord, []byte, error) {
	if _, err := t.term.Write([]byte(vt100GetCursorActivePos)); err != nil {
		return nil, nil, err
	}

	r := &readWrapper{
		term: t.term,
	}

	maxRead := 1 << 24
	return readUntilCursorPosition(r, maxRead)
}

type readWrapper struct {
	term terminal.Terminal
}

func (r *readWrapper) Read(b []byte) (int, error) {
	return r.term.UnsafeRead(b)
}

func readUntilCursorPosition(r io.Reader, maxRead int) (*TermCoord, []byte, error) {
	var buffer []byte
	tempBuf := make([]byte, 1)
	cursorPosRegex := regexp.MustCompile(`\x1b\[(\d+);(\d+)R`)

	var coord TermCoord

	for {
		if len(tempBuf) > maxRead {
			return nil, buffer, errors.New("Failed to get CursorPosition before maxRead hit")
		}
		n, err := r.Read(tempBuf)
		if err != nil {
			if n > 0 {
				buffer = append(buffer, tempBuf[:n]...)
			}
			return nil, buffer, err
		}

		buffer = append(buffer, tempBuf[0])

		if matches := cursorPosRegex.FindSubmatch(buffer); matches != nil {
			coord.Row, _ = strconv.Atoi(string(matches[1]))
			coord.Col, _ = strconv.Atoi(string(matches[2]))
			extraBytes := buffer[:len(buffer)-len(matches[0])]

			return &coord, extraBytes, nil
		}
	}
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

func (t *VT100) MoveToCoord(coord TermCoord) {
	t.term.Write([]byte(fmt.Sprintf(vt100CursorPosition, coord.Row, coord.Col)))
}

func (t *VT100) ClearToEndOfLine() {
	t.term.Write([]byte(vt100ClearToEndOfLine))
}

func (t *VT100) ScrollUp() {
	t.term.Write([]byte(vtPanDown))
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

	vtPanDown = "\x1b[S" // scroll up

	// ctrlA = 0x01
	// ctrlB = 0x02
	// ctrlC = 0x03
	// ctrlD = 0x04
	// ctrlE = 0x05
	// ctrlL = 0x0C
)
