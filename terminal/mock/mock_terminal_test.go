package mock

import (
	"fmt"
	"testing"

	"github.com/psanford/hat/vt100"
)

func TestMock(t *testing.T) {

	cols := 80
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

	vt.Write([]byte("hi there\r\n"))
	vt.Write([]byte("line 2\r\n"))
	vt.Write([]byte("line 3\r\n"))

	fmt.Println("terminal:")
	for _, line := range term.display.Lines {
		for _, c := range line {
			fmt.Printf("%c", c.Code)
		}
		fmt.Println()
	}
	fmt.Println()

	fmt.Printf("stdout: %s\n", term.stdout.Bytes())

}
