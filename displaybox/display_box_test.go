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

type TestCase struct {
	name       string
	action     func(d *DisplayBox)
	expect     []string
	withBorder []string
}

func TestDisplayBox(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert 'hi'",
			action: func(d *DisplayBox) {
				d.Insert([]byte("hi"))
			},
			expect: []string{
				"hi         ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~hi       ~",
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
		{
			name: "Move right, down and insert '%'",
			action: func(d *DisplayBox) {
				d.MvRight()
				d.MvDown()
				d.Insert([]byte("%"))
			},
			expect: []string{
				"h#$i       ",
				"@.2%       ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#$i     ~",
				"~@.2%     ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move right (nop), down (nop) and insert '^'",
			action: func(d *DisplayBox) {
				d.MvRight()
				d.MvDown()
				d.Insert([]byte("^"))
			},
			expect: []string{
				"h#$i       ",
				"@.2%^      ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#$i     ~",
				"~@.2%^    ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Backspace",
			action: func(d *DisplayBox) {
				d.Backspace()
			},
			expect: []string{
				"h#$i       ",
				"@.2%       ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#$i     ~",
				"~@.2%     ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Del (nop)",
			action: func(d *DisplayBox) {
				d.Del()
			},
			expect: []string{
				"h#$i       ",
				"@.2%       ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~h#$i     ~",
				"~@.2%     ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Move Left, Del",
			action: func(d *DisplayBox) {
				d.MvLeft()
				d.Del()
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

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestMoveUp(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abcd",
			action: func(d *DisplayBox) {
				d.Insert([]byte("abcd"))
			},
			expect: []string{
				"abcd       ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~abcd     ~",
				"~~~~       ",
				"           ",
				"           ",
			},
		},
		{
			name: "Insert new line",
			action: func(d *DisplayBox) {
				d.InsertNewline()
			},
			expect: []string{
				"abcd       ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~abcd     ~",
				"~         ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "MvUp, insert 1",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.Insert([]byte("1"))
			},
			expect: []string{
				"1abcd      ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~1abcd    ~",
				"~         ~",
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
				expect := tc.expect

				if border {
					db = dBorder
					term = termBorder
					expect = tc.withBorder
				}

				tc.action(db)

				buf := new(bytes.Buffer)
				for _, line := range expect {
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

func TestBolEol(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abcd",
			action: func(d *DisplayBox) {
				d.Insert([]byte("abcd"))
				d.InsertNewline()
				d.Insert([]byte("1234"))
			},
			expect: []string{
				"abcd       ",
				"1234       ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~abcd     ~",
				"~1234     ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "EOL, insert 5, BOL, insert 6, EOL, insert 7",
			action: func(d *DisplayBox) {
				d.MvEOL()
				d.Insert([]byte("5"))
				d.MvBOL()
				d.Insert([]byte("6"))
				d.MvEOL()
				d.Insert([]byte("7"))
			},
			expect: []string{
				"abcd       ",
				"6123457    ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~abcd     ~",
				"~6123457  ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "MvUp, insert f, EOL, insert g, BOL, insert h, EOL, insert i",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.Insert([]byte("f"))
				d.MvEOL()
				d.Insert([]byte("g"))
				d.MvBOL()
				d.Insert([]byte("h"))
				d.MvEOL()
				d.Insert([]byte("i"))
			},
			expect: []string{
				"habcdfgi   ",
				"6123457    ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~habcdfgi ~",
				"~6123457  ~",
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
				expect := tc.expect

				if border {
					db = dBorder
					term = termBorder
					expect = tc.withBorder
				}

				tc.action(db)

				buf := new(bytes.Buffer)
				for _, line := range expect {
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

func TestBackspaceAcrossLines(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abc",
			action: func(d *DisplayBox) {
				d.Insert([]byte("a"))
				d.InsertNewline()
				d.Insert([]byte("b"))
				d.InsertNewline()
				d.Insert([]byte("c"))
			},
			expect: []string{
				"a          ",
				"b          ",
				"c          ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~c        ~",
				"~~~~       ",
			},
		},
		{
			name: "MvUp, backspace",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.Backspace()
			},
			expect: []string{
				"a          ",
				"           ",
				"c          ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~c        ~",
				"~~~~       ",
			},
		},
		{
			name: "backspace",
			action: func(d *DisplayBox) {
				d.Backspace()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"a          ",
				"c          ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~c        ~",
				"~~~~       ",
				"~~~~       ",
			},
		},
		{
			name: "mvLeft, backspace (noop), mvDown",
			action: func(d *DisplayBox) {
				d.MvLeft()
				d.Backspace()
				d.MvDown()
			},
			expect: []string{
				"a          ",
				"c          ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~c        ~",
				"~~~~       ",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestBackspaceEndOfBuffer(t *testing.T) {
	width := 11
	height := 6

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abc",
			action: func(d *DisplayBox) {
				d.Insert([]byte("a"))
				d.InsertNewline()
				d.Insert([]byte("b"))
				d.InsertNewline()
				d.Insert([]byte("c"))
				d.InsertNewline()
			},
			expect: []string{
				"a          ",
				"b          ",
				"c          ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~c        ~",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "backspace",
			action: func(d *DisplayBox) {
				d.Backspace()
			},
			expect: []string{
				"a          ",
				"b          ",
				"c          ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~c        ~",
				"~~~~       ",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestPartialTermScrollUp(t *testing.T) {
	width := 11
	height := 6
	otherAppRows := 2

	dNoBorder, termNoBorder := setupMockPartialTerm(width, height, otherAppRows, false)
	dBorder, termBorder := setupMockPartialTerm(width, height+2, otherAppRows, true)

	testCases := []TestCase{
		{
			name: "Insert abc",
			action: func(d *DisplayBox) {
				d.Insert([]byte("a"))
				d.InsertNewline()
				d.Insert([]byte("b"))
				d.InsertNewline()
				d.Insert([]byte("c"))
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"a          ",
				"b          ",
				"c          ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~c        ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "move up, insert newline",
			action: func(d *DisplayBox) {
				d.MvUp()
				d.InsertNewline()
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"a          ",
				"b          ",
				"           ",
				"c          ",
			},
			withBorder: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~         ~",
				"~c        ~",
				"~~~~       ",
			},
		},
		{
			name: "insert newline",
			action: func(d *DisplayBox) {
				d.InsertNewline()
			},
			expect: []string{
				"=OTHER 1=  ",
				"a          ",
				"b          ",
				"           ",
				"           ",
				"c          ",
			},
			withBorder: []string{
				"=OTHER 1=  ",
				"~~~~       ",
				"~a        ~",
				"~b        ~",
				"~         ~",
				"~         ~",
				"~c        ~",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestStartAtBottom(t *testing.T) {
	width := 11
	height := 6
	otherAppRows := 5

	dNoBorder, termNoBorder := setupMockPartialTerm(width, height, otherAppRows, false)
	dBorder, termBorder := setupMockPartialTerm(width, height, otherAppRows, true)

	testCases := []TestCase{
		{
			name: "Insert a",
			action: func(d *DisplayBox) {
				d.Insert([]byte("a"))
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"a          ",
			},
			withBorder: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~a        ~",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func checkResults(t *testing.T, tc []TestCase, dNoBorder, dBorder *DisplayBox, termNoBorder, termBorder *mock.MockTerm) {
	t.Helper()

	for _, border := range []bool{false, true} {
		for _, tc := range tc {
			t.Run(fmt.Sprintf("%s border=%t", tc.name, border), func(t *testing.T) {
				db := dNoBorder
				term := termNoBorder
				expect := tc.expect

				if border {
					db = dBorder
					term = termBorder
					expect = tc.withBorder
				}

				tc.action(db)

				buf := new(bytes.Buffer)
				for _, line := range expect {
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

func setupMockPartialTerm(width, height, inuseRows int, showBorder bool) (*DisplayBox, *mock.MockTerm) {
	term := mock.NewMock(width, height)
	vt := vt100.New(term)
	gb := gapbuffer.New(2)

	for i := 0; i < inuseRows; i++ {
		vt.Write([]byte(fmt.Sprintf("=OTHER %d=", i)))
		vt.MoveTo(i+2, 1)
	}

	d := New(vt, gb, showBorder)
	return d, term
}

const resetSeq = "\x1b[0m"
