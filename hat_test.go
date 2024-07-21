package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/psanford/hat/terminal"
	"github.com/psanford/hat/terminal/mock"
)

func TestEditor(t *testing.T) {
	term := mock.NewMock(11, 5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ed, in := newTestEditor(ctx, term)

	go ed.run(ctx)

	in.WriteString("1")
	in.WriteString("\r")
	in.WriteString("2\r")
	in.WriteString("3")

	var expect bytes.Buffer

	checkResult := func() {
		t.Helper()
		var screenBuf bytes.Buffer
		err := term.Render(&screenBuf)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(screenBuf.Bytes(), expect.Bytes()) {
			fmt.Printf("got:\n%s", hex.Dump(screenBuf.Bytes()))
			fmt.Printf("expect:\n%s", hex.Dump(expect.Bytes()))
			t.Fatal("buffer mismatch")
		}
	}

	fmt.Fprintf(&expect, "1          %s\n", resetSeq)
	fmt.Fprintf(&expect, "2          %s\n", resetSeq)
	fmt.Fprintf(&expect, "3          %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)
	checkResult()

	in.WriteControl(vt100CursorLeft)
	in.WriteString("4")

	expect = bytes.Buffer{}
	fmt.Fprintf(&expect, "1          %s\n", resetSeq)
	fmt.Fprintf(&expect, "2          %s\n", resetSeq)
	fmt.Fprintf(&expect, "43         %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)
	checkResult()

	in.WriteControl(vt100CursorUp)
	in.WriteString("5")

	expect = bytes.Buffer{}
	fmt.Fprintf(&expect, "1          %s\n", resetSeq)
	fmt.Fprintf(&expect, "25         %s\n", resetSeq)
	fmt.Fprintf(&expect, "43         %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)
	checkResult()

	in.WriteString("\r")
	in.WriteString("6")

	expect = bytes.Buffer{}
	fmt.Fprintf(&expect, "1          %s\n", resetSeq)
	fmt.Fprintf(&expect, "25         %s\n", resetSeq)
	fmt.Fprintf(&expect, "6          %s\n", resetSeq)
	fmt.Fprintf(&expect, "43         %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)
	checkResult()

}

const resetSeq = "\x1b[0m"

type ioTracker struct {
	sent    uint32
	recv    uint32
	w       io.Writer
	evtChan chan struct{}

	sendDone chan struct{}
	done     chan struct{}
}

func (t *ioTracker) WriteString(s string) (int, error) {
	n, err := t.w.Write([]byte(s))
	for i := 0; i < n; i++ {
		<-t.evtChan
	}
	return n, err
}

func (t *ioTracker) WriteControl(s string) (int, error) {
	n, err := t.w.Write([]byte(s))
	<-t.evtChan
	return n, err
}

func newTestEditor(ctx context.Context, term terminal.Terminal) (*editor, *ioTracker) {
	inPipe, in := io.Pipe()
	ed := newEditor(inPipe, io.Discard, io.Discard, term)

	tracker := &ioTracker{
		w:        in,
		evtChan:  make(chan struct{}, 10),
		sendDone: make(chan struct{}),
		done:     make(chan struct{}),
	}

	ed.debugEventCh = tracker.evtChan

	return ed, tracker
}

const (
	vt100CursorUp    = "\x1b[A"
	vt100CursorDown  = "\x1b[B"
	vt100CursorRight = "\x1b[C"
	vt100CursorLeft  = "\x1b[D"
)
