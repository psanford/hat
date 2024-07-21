package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/psanford/hat/terminal"
	"github.com/psanford/hat/terminal/mock"
)

func TestEditor(t *testing.T) {
	term := mock.NewMock(10, 5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ed, in := newTestEditor(ctx, term)

	go ed.run(ctx)

	in.WriteString("1\r")
	in.WriteString("2\r")
	in.WriteString("3\r")

	in.Wait()

	var screenBuf bytes.Buffer
	err := term.Render(&screenBuf)
	if err != nil {
		t.Fatal(err)
	}

	var expect bytes.Buffer
	fmt.Fprintf(&expect, "1         %s\n", resetSeq)
	fmt.Fprintf(&expect, "2         %s\n", resetSeq)
	fmt.Fprintf(&expect, "3         %s\n", resetSeq)
	fmt.Fprintf(&expect, "          %s\n", resetSeq)
	fmt.Fprintf(&expect, "          %s", resetSeq)

	if !bytes.Equal(screenBuf.Bytes(), expect.Bytes()) {
		fmt.Printf("got:\n%s", hex.Dump(screenBuf.Bytes()))
		fmt.Printf("expect:\n%s", hex.Dump(expect.Bytes()))
		t.Fatal(cmp.Diff(screenBuf.Bytes(), expect.Bytes()))
	}


	in.Reset(ctx)


	in.WriteControl(s string)


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
	atomic.AddUint32(&t.sent, uint32(n))
	return n, err
}

func (t *ioTracker) WriteControl(s string) (int, error) {
	n, err := t.w.Write([]byte(s))
	atomic.AddUint32(&t.sent, 1)
	return n, err
}

func (t *ioTracker) watchEvents(ctx context.Context) {
	var stopOnEqual bool
	for {
		select {
		case <-t.evtChan:
			recv := atomic.AddUint32(&t.recv, 1)
			if stopOnEqual {
				sent := atomic.LoadUint32(&t.sent)
				if recv == sent {
					close(t.done)
					return
				}
			}
		case <-t.sendDone:
			stopOnEqual = true
			t.sendDone = nil

			recv := atomic.LoadUint32(&t.recv)
			sent := atomic.LoadUint32(&t.sent)
			if recv == sent {
				close(t.done)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (t *ioTracker) Wait() {
	close(t.sendDone)
	<-t.done
}

func (t *ioTracker) Reset(ctx context.Context) {
	t.sendDone = make(chan struct{})
	t.done = make(chan struct{})
	t.recv = 0
	t.sent = 0

	go t.watchEvents(ctx)

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

	go tracker.watchEvents(ctx)

	return ed, tracker
}
