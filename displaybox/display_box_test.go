package displaybox

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/terminal/mock"
	"github.com/psanford/hat/vt100"
)

type TestCase struct {
	name       string
	action     func(d *DisplayBox, term *mock.MockTerm)
	expect     []string
	withBorder []string
}

func TestMain(m *testing.M) {
	overflowBorderTop = []byte("^^^^")
	overflowBorderBottom = []byte("@@@@")
	overflowBorderLeft = []byte("<")
	overflowBorderRight = []byte(">")

	code := m.Run()
	os.Exit(code)
}

func TestDisplayBox(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert 'hi'",
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestBolEol(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abcd",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Insert([]byte("abcd\n1234"))
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestBackspaceAcrossLines(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert abc",
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
			action: func(d *DisplayBox, term *mock.MockTerm) {
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
		{
			name: "Insert newline",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.InsertNewline()
				d.InsertNewline()
			},
			expect: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"a          ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 4=  ",
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "backspace",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Backspace()
			},
			expect: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"a          ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 4=  ",
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~~~~       ",
				"~~~~       ",
			},
		},
		{
			name: "insert newline",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.InsertNewline()
			},
			expect: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"a          ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 4=  ",
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~         ~",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestOverflowTopBottom(t *testing.T) {
	width := 11
	height := 6

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height+2, true)

	testCases := []TestCase{
		{
			name: "Fill screen",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 0; i < height; i++ {
					d.Insert([]byte(fmt.Sprintf("%d", i)))
					if i < height-1 {
						d.InsertNewline()
					}
				}
			},
			expect: []string{
				"0          ",
				"1          ",
				"2          ",
				"3          ",
				"4          ",
				"5          ",
			},
			withBorder: []string{
				"~~~~       ",
				"~0        ~",
				"~1        ~",
				"~2        ~",
				"~3        ~",
				"~4        ~",
				"~5        ~",
				"~~~~       ",
			},
		},
		{
			name: "New line",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.InsertNewline()
			},
			expect: []string{
				"1          ",
				"2          ",
				"3          ",
				"4          ",
				"5          ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~1        ~",
				"~2        ~",
				"~3        ~",
				"~4        ~",
				"~5        ~",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Move to copt of current viewport",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 0; i < height-1; i++ {
					d.MvUp()
				}
			},
			expect: []string{
				"1          ",
				"2          ",
				"3          ",
				"4          ",
				"5          ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~1        ~",
				"~2        ~",
				"~3        ~",
				"~4        ~",
				"~5        ~",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Trigger Scroll up",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvUp()
				d.MvUp() // nop
			},
			expect: []string{
				"0          ",
				"1          ",
				"2          ",
				"3          ",
				"4          ",
				"5          ",
			},
			withBorder: []string{
				"~~~~       ",
				"~0        ~",
				"~1        ~",
				"~2        ~",
				"~3        ~",
				"~4        ~",
				"~5        ~",
				"@@@@       ",
			},
		},
		{
			name: "Move back down to bottom",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 0; i < height; i++ {
					d.MvDown()
				}
			},
			expect: []string{
				"1          ",
				"2          ",
				"3          ",
				"4          ",
				"5          ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~1        ~",
				"~2        ~",
				"~3        ~",
				"~4        ~",
				"~5        ~",
				"~         ~",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestOverflowRight(t *testing.T) {
	width := 11
	height := 4

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width+2, height, true)

	testCases := []TestCase{
		{
			name: "Fill screen",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 'a'; i < 'a'+rune(width)+2; i++ {
					d.Insert([]byte(string([]rune{i})))
				}
			},
			expect: []string{
				"defghijklm ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"<defghijklm ~",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "Scroll left by two chars",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 'a'; i < 'a'+rune(width); i++ {
					d.MvLeft()
				}
			},
			expect: []string{
				"cdefghijkl ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"<cdefghijkl >",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "Mv right (no scroll) and insert",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvRight()
				d.Insert([]byte("!"))
			},
			expect: []string{
				"c!defghijk ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"<c!defghijk >",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "Scroll left full",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvLeft()
				d.MvLeft()
				d.MvLeft()
				d.MvLeft()
			},
			expect: []string{
				"abc!defghi ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"~abc!defghi >",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "eol",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvEOL()
			},
			expect: []string{
				"defghijklm ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"<defghijklm ~",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "bol",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvBOL()
			},
			expect: []string{
				"abc!defghi ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"~abc!defghi >",
				"~~~~         ",
				"             ",
			},
		},
		{
			name: "right two, newline",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvRight()
				d.MvRight()
				d.InsertNewline()
			},
			expect: []string{
				"ab         ",
				"c!defghijk ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~         ",
				"~ab         ~",
				"~c!defghijk >",
				"~~~~         ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestLastOwnedRow(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Insert lines",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Insert([]byte("a\nb\nc\nd\n"))
				d.MvUp()
				d.MvUp()
			},
			expect: []string{
				"a          ",
				"b          ",
				"c          ",
				"d          ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~c        ~",
				"~d        ~",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "mvLastOwnedLine",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				tc := d.LastOwnedRow()
				if tc.Row != height+1 {
					panic(fmt.Sprintf("expected row == height+1 but was r=%d h=%d", tc.Row, height))
				}
			},
			expect: []string{
				"a          ",
				"b          ",
				"c          ",
				"d          ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~c        ~",
				"~d        ~",
				"~         ~",
				"~~~~       ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestEOLAtEndOfFile(t *testing.T) {
	// If the file ends with \n\n and you are on the first
	// newline, make sure MvRight and EOL work proplery.
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Check content",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Insert([]byte("a\n"))
			},
			expect: []string{
				"a          ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "mv up, eol",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvUp()
				d.MvEOL()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"a          ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "bol, right",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvBOL()
				d.MvRight()
				d.MvRight()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"a          ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~a        ~",
				"~         ~",
				"~~~~       ",
				"           ",
			},
		},
	}

	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestTerminalResize(t *testing.T) {
	width := 11
	height := 5

	dNoBorder, termNoBorder := setupMock(width, height, false)
	dBorder, termBorder := setupMock(width, height, true)

	testCases := []TestCase{
		{
			name: "Fill screen",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Insert([]byte("abcdefghijklm\n"))
				d.Insert([]byte("1234567890123\n"))
				d.Insert([]byte("ABCDEFGHIJKLM\n"))
				d.Insert([]byte("0987654321098\n"))
				d.Insert([]byte("nopqrstuvwxyz"))
			},
			expect: []string{
				"abcdefghij ",
				"1234567890 ",
				"ABCDEFGHIJ ",
				"0987654321 ",
				"qrstuvwxyz ",
			},
			withBorder: []string{
				"^^^^       ",
				"~ABCDEFGH >",
				"~09876543 >",
				"<stuvwxyz ~",
				"~~~~       ",
			},
		},
		{
			name: "Grow right 1",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width+1, height)
				d.TerminalResize()
				d.cursorPosSanityCheck()
				d.Insert([]byte("!"))
			},
			expect: []string{
				"abcdefghijk ",
				"12345678901 ",
				"ABCDEFGHIJK ",
				"09876543210 ",
				"qrstuvwxyz! ",
			},
			withBorder: []string{
				"^^^^        ",
				"~ABCDEFGHI >",
				"~098765432 >",
				"<stuvwxyz! ~",
				"~~~~        ",
			},
		},
		{
			name: "Grow down 1",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width+1, height+1)
				d.TerminalResize()
				d.cursorPosSanityCheck()
				d.Insert([]byte("@"))
			},
			expect: []string{
				"abcdefghijk ",
				"12345678901 ",
				"ABCDEFGHIJK ",
				"09876543210 ",
				"rstuvwxyz!@ ",
				"            ",
			},
			withBorder: []string{
				"^^^^        ",
				"~ABCDEFGHI >",
				"~098765432 >",
				"<tuvwxyz!@ ~",
				"~~~~        ",
				"~~~~        ",
			},
		},
		{
			name: "Shrink left 3",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width-2, height+1)
				d.TerminalResize()
				d.cursorPosSanityCheck()
				d.Insert([]byte("#"))
			},
			expect: []string{
				"abcdefgh ",
				"12345678 ",
				"ABCDEFGH ",
				"09876543 ",
				"vwxyz!@# ",
				"         ",
			},
			withBorder: []string{
				"^^^^     ",
				"~ABCDEF >",
				"~098765 >",
				"<xyz!@# ~",
				"~~~~     ",
				"~~~~     ",
			},
		},
		{
			name: "bol, shrink left 1 more",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.MvBOL()
				term.Resize(width-3, height+1)
				d.TerminalResize()
				d.cursorPosSanityCheck()
				d.Insert([]byte("$"))
			},
			expect: []string{
				"abcdefg ",
				"1234567 ",
				"ABCDEFG ",
				"0987654 ",
				"$nopqrs ",
				"        ",
			},
			withBorder: []string{
				"^^^^    ",
				"~ABCDE >",
				"~09876 >",
				"~$nopq >",
				"~~~~    ",
				"~~~~    ",
			},
		},
		{
			name: "redraw borders",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				for i := 0; i < 4; i++ {
					d.MvUp()
				}
				for i := 0; i < 4; i++ {
					d.MvDown()
				}
				d.cursorPosSanityCheck()
				d.MvLeft()
				d.Insert([]byte("%"))
			},
			expect: []string{
				"abcdefg ",
				"1234567 ",
				"ABCDEFG ",
				"0987654 ",
				"%$nopqr ",
				"        ",
			},
			withBorder: []string{
				"^^^^    ",
				"~12345 >",
				"~ABCDE >",
				"~09876 >",
				"~%$nop >",
				"~~~~    ",
			},
		},
		{
			name: "shrink bottom",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width-3, height-1)
				d.TerminalResize()
				d.cursorPosSanityCheck()
				d.MvBOL()
				d.Insert([]byte("^"))
			},
			expect: []string{
				"ABCDEFG ",
				"0987654 ",
				"^%$nopq ",
				"        ",
			},
			withBorder: []string{
				"^^^^    ",
				"~09876 >",
				"~^%$no >",
				"~~~~    ",
			},
		},
	}
	checkResults(t, testCases, dNoBorder, dBorder, termNoBorder, termBorder)
}

func TestTerminalRowShrink(t *testing.T) {
	width := 11
	height := 10
	otherAppRows := 5

	dNoBorder, termNoBorder := setupMockPartialTerm(width, height, otherAppRows, false)
	dBorder, termBorder := setupMockPartialTerm(width, height, otherAppRows, true)

	testCases := []TestCase{
		{
			name: "Fill screen",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				d.Insert([]byte("abcdefghijklm\n"))
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"abcdefghij ",
				"           ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
				"           ",
			},
		},
		{
			name: "Shrink -1",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-1)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"abcdefghij ",
				"           ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -2",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-2)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"abcdefghij ",
				"           ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -3",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-3)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			expect: []string{
				"=OTHER 0=  ",
				"=OTHER 1=  ",
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"abcdefghij ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -4",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-4)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			// PMS: This is obviously wrong, but for now the test is documenting the current behavior
			expect: []string{
				"=OTHER 2=  ",
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"abcdefghij ",
				"abcdefghij ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 3=  ",
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -5",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-5)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			// PMS: This is obviously wrong, but for now the test is documenting the current behavior
			expect: []string{
				"=OTHER 4=  ",
				"abcdefghij ",
				"abcdefghij ",
				"abcdefghij ",
				"           ",
			},
			withBorder: []string{
				"=OTHER 4=  ",
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -6",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-6)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			// PMS: This is obviously wrong, but for now the test is documenting the current behavior
			expect: []string{
				"abcdefghij ",
				"abcdefghij ",
				"abcdefghij ",
				"           ",
			},
			withBorder: []string{
				"~~~~       ",
				"~abcdefgh >",
				"~         ~",
				"~~~~       ",
			},
		},
		{
			name: "Shrink -7",
			action: func(d *DisplayBox, term *mock.MockTerm) {
				term.Resize(width, height-7)
				d.TerminalResize()
				d.cursorPosSanityCheck()
			},
			// PMS: This is obviously wrong, but for now the test is documenting the current behavior
			expect: []string{
				"abcdefghij ",
				"abcdefghij ",
				"           ",
			},
			withBorder: []string{
				"^^^^       ",
				"~         ~",
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

				tc.action(db, term)

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
