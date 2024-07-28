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
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, false)

	testCases := []struct {
		name       string
		action     func(d *DisplayBox)
		expect     []string
		withBorder []string
	}{
		{
			name:   "Insert 'hi'",
			action: func(d *DisplayBox) { d.Insert([]byte("hi")) },
			expect: []string{
				"hi         ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~ hi      ~",
				"~~~~       ",
				"           ",
				"           ",
			},
		},
		{
			name: "Insert newline and '2'",
			action: func(d *DisplayBox) {
				d.InsertNewline()
				d.Insert([]byte("2"))
			},
			expect: []string{
				"hi         ",
				"2          ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~hi       ~",
				"~2        ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move left and insert '.'",
			action: func(d *DisplayBox) {
				d.MvLeft()
				d.Insert([]byte("."))
			},
			expect: []string{
				"hi         ",
				".2         ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~hi       ~",
				"~.2       ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move left 4 times and insert '@'",
			action: func(d *DisplayBox) {
				d.MvLeft()
				d.MvLeft()
				d.MvLeft()
				d.MvLeft()
				d.Insert([]byte("@"))
			},
			expect: []string{
				"hi         ",
				"@.2        ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~hi       ~",
				"~@.2      ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move up and insert '#'",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.Insert([]byte("#"))
			},
			expect: []string{
				"h#i        ",
				"@.2        ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#i      ~",
				"~@.2      ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move up (nop) and insert '$'",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.Insert([]byte("$"))
			},
			expect: []string{
				"h#$i       ",
				"@.2        ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#$i     ~",
				"~@.2      ~",
				"~~~~       ",
				"           ",
			},
		},
	}

	for _, border := range []bool{false, true} {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s border=%t", tc.name, border), func(t *testing.T) {
				db := dNoBorder
				term := termNoBorder

				if border {
					db = dBorder
					term = termBorder
				}

				tc.action(db)

				buf := new(bytes.Buffer)
				for _, line := range tc.expect {
					buf.Write([]byte(line))
					buf.Write([]byte(resetSeq))
					buf.Write([]byte("\n"))
				}

				buf.Truncate(buf.Len() - 1)

				checkResult(t, term, buf)
			})
		}
	}

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

func setupMock(width, height int, showBorder bool) (*DisplayBox, *mock.MockTerm) {
	term := mock.NewMock(width, height)
	vt := vt100.New(term)
	gb := gapbuffer.New(2)

	d := New(vt, gb, showBorder)
	return d, term
}

const resetSeq = "\x1b[0m"
