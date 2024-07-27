package displaybox

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/terminal/mock"
	"github.com/psanford/hat/vt100"
)

func TestDisplayBox(t *testing.T) {
	term := mock.NewMock(11, 5)
	vt := vt100.New(term)
	gb := gapbuffer.New(2)

	d := New(vt, gb, false)

	d.Insert([]byte("hi"))

	var expect bytes.Buffer
	fmt.Fprintf(&expect, "hi         %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)

	d.InsertNewline()
	d.Insert([]byte("2"))

	expect.Reset()
	fmt.Fprintf(&expect, "hi         %s\n", resetSeq)
	fmt.Fprintf(&expect, "2          %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)

	checkResult(t, term, &expect)

	term = mock.NewMock(11, 5)
	vt = vt100.New(term)
	gb = gapbuffer.New(2)
	d = New(vt, gb, true)

	expect.Reset()
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "~         ~%s\n", resetSeq)
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)

	checkResult(t, term, &expect)

	d.Insert([]byte("hi"))

	expect.Reset()
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "~hi       ~%s\n", resetSeq)
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)

	checkResult(t, term, &expect)

	d.InsertNewline()
	d.Insert([]byte("2"))

	expect.Reset()
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "~hi       ~%s\n", resetSeq)
	fmt.Fprintf(&expect, "~2        ~%s\n", resetSeq)
	fmt.Fprintf(&expect, "~~~~       %s\n", resetSeq)
	fmt.Fprintf(&expect, "           %s", resetSeq)

	checkResult(t, term, &expect)

}

func checkResult(t *testing.T, term *mock.MockTerm, expect *bytes.Buffer) {
	t.Helper()
	var screenBuf bytes.Buffer
	err := term.Render(&screenBuf)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(screenBuf.Bytes(), expect.Bytes()) {
		fmt.Printf("got:\n%s", hex.Dump(screenBuf.Bytes()))
		fmt.Printf("expect:\n%s", hex.Dump(expect.Bytes()))
		t.Error("buffer mismatch")
	}
}

const resetSeq = "\x1b[0m"
