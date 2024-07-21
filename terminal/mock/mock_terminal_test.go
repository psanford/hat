package mock

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/psanford/hat/vt100"
)

const resetSeq = "\x1b[0m"

func TestMock(t *testing.T) {

	cols := 10
	rows := 5
	term := NewMock(cols, rows)

	gotCols, gotRows := term.Size()
	if cols != gotCols {
		t.Errorf("cols: expected=%d got=%d", cols, gotCols)
	}
	if rows != gotRows {
		t.Errorf("rows: expected=%d got=%d", rows, gotRows)
	}

	vt := vt100.New(term)

	vt.Write([]byte("first line\r\n"))
	for i := 0; i < 10; i++ {
		fmt.Fprintf(vt, "line %d\r\n", i)
	}
	vt.Write([]byte("last line\r\n"))

	var screenBuf bytes.Buffer
	err := term.Render(&screenBuf)
	if err != nil {
		t.Fatal(err)
	}

	var expect bytes.Buffer
	fmt.Fprintf(&expect, "line 6    %s\n", resetSeq)
	fmt.Fprintf(&expect, "line 7    %s\n", resetSeq)
	fmt.Fprintf(&expect, "line 8    %s\n", resetSeq)
	fmt.Fprintf(&expect, "line 9    %s\n", resetSeq)
	fmt.Fprintf(&expect, "last line %s", resetSeq)

	if !bytes.Equal(screenBuf.Bytes(), expect.Bytes()) {
		t.Fatal(cmp.Diff(screenBuf.Bytes(), expect.Bytes()))
	}
}
