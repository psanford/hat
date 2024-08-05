package mock

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vito/midterm"
)

type MockTerm struct {
	cols int
	rows int

	term *midterm.Terminal

	stdout *bytes.Buffer

	ctrlBuf bytes.Buffer
	ctrlCh  chan []byte
}

func NewMock(cols, rows int) *MockTerm {
	var stdout bytes.Buffer

	t := midterm.NewTerminal(rows, cols)

	pr0, pw0 := io.Pipe()

	ctrlCh := make(chan []byte, 1)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := pr0.Read(buf)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(fmt.Sprintf("pr0 read err: %s", err))
			}

			if n < 1 {
				continue
			}
			got := buf[:n]

			sendBuf := make([]byte, n)
			copy(sendBuf, got)

			select {
			case ctrlCh <- sendBuf:
			default:
				panic("Would block sending to ctrlCh")
			}
		}
	}()

	t.ForwardResponses = pw0
	t.Raw = true

	return &MockTerm{
		cols: cols,
		rows: rows,

		term:   t,
		stdout: &stdout,
		ctrlCh: ctrlCh,
	}
}

func (t *MockTerm) EnableRawMode() {
}

func (t *MockTerm) Restore() {
}

func (t *MockTerm) ReadControl() ([]byte, error) {
	return <-t.ctrlCh, nil
}

func (t *MockTerm) Write(b []byte) (int, error) {
	return t.term.Write(b)
}

func (t *MockTerm) Size() (cols, rows int) {
	return t.term.Width, t.term.Height
}

func (t *MockTerm) Render(w io.Writer) error {
	return t.term.Render(w)
}

func (t *MockTerm) Resize(cols, rows int) {
	t.term.Resize(rows, cols)
}
