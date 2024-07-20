package mock

import (
	"bytes"
	"io"

	"github.com/markkurossi/vt100"
)

type MockTerm struct {
	cols int
	rows int

	display *vt100.Display
	emu     *vt100.Emulator

	stdout *bytes.Buffer
}

func NewMock(cols, rows int) *MockTerm {
	var stdout bytes.Buffer
	display := vt100.NewDisplay(cols, rows)
	emulator := vt100.NewEmulator(&stdout, io.Discard, display)

	return &MockTerm{
		cols: cols,
		rows: rows,

		display: display,
		emu:     emulator,
		stdout:  &stdout,
	}
}

func (t *MockTerm) EnableRawMode() {
}

func (t *MockTerm) Restore() {
}

func (t *MockTerm) Read(b []byte) (int, error) {
	return 0, nil
}

func (t *MockTerm) Write(b []byte) (int, error) {
	for _, c := range b {
		t.emu.Input(int(c))
	}
	return len(b), nil
}

func (t *MockTerm) Size() (cols, rows int) {
	p := t.display.Size()

	return p.X, p.Y
}
