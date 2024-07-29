package displaybox

import (
	"bytes"
	"fmt"
	"io"

	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/vt100"
)

type DisplayBox struct {
	editableRows  int
	termOwnedRows int

	vt100         *vt100.VT100
	boarderTop    int
	boarderLeft   int
	boarderRight  int
	boarderBottom int
	addBorder     bool
	buf           *gapbuffer.GapBuffer

	termSize  vt100.TermCoord
	firstRowT int

	// zero indexed location of the cursor within the editable area
	cursorCoord *viewPortCoord
}

func New(term *vt100.VT100, gb *gapbuffer.GapBuffer, addBorder bool) *DisplayBox {
	cursorT := term.CursorPos()

	d := &DisplayBox{
		editableRows:  1,
		termOwnedRows: 1,

		vt100:       term,
		buf:         gb,
		cursorCoord: &viewPortCoord{},
		termSize:    term.Size(),
		firstRowT:   cursorT.Row,
	}

	if addBorder {
		d.boarderTop = 1
		d.boarderLeft = 1
		d.boarderRight = 1
		d.boarderBottom = 1
		d.termOwnedRows = 3
		d.vt100.Write([]byte("~~~~"))
		d.vt100.Write([]byte("\r\n"))
		d.vt100.Write([]byte("~"))
		for i := 2; i < d.termSize.Col; i++ {
			d.vt100.Write([]byte(" "))
		}
		d.vt100.Write([]byte("~"))
		d.vt100.Write([]byte("\r\n"))
		d.vt100.Write([]byte("~~~~"))
		pos := d.vt100.CursorPos()
		d.vt100.MoveTo(pos.Row-1, 2)
	}
	return d
}

func (d *DisplayBox) MvLeft() {
	d.cursorPosSanityCheck()
	if d.cursorCoord.X > 0 {

		d.buf.Seek(-1, io.SeekCurrent)
		d.cursorCoord.X--

		d.redrawCursor()
	}
}

func (d *DisplayBox) MvRight() {
	d.cursorPosSanityCheck()

	bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
	endBufPos := d.buf.Size()

	// if we're at the very end of the file our cursor should be at lastpos+1
	_, eolPos := d.buf.GetLine(0)
	if eolPos == endBufPos-1 {
		eolPos = endBufPos
	}
	if bufPos == int64(eolPos) {
		return
	}
	_, err := d.buf.Seek(1, io.SeekCurrent)
	if err != nil {
		panic(fmt.Sprintf("MvRight seek forward unexepected error: %s", err))
	}

	d.cursorCoord.X++
	d.redrawCursor()
}

func (d *DisplayBox) MvUp() {
	d.cursorPosSanityCheck()

	prevStart, prevEnd := d.buf.GetLine(-1)
	if prevStart == -1 && prevEnd == -1 {
		// we're on the first line
		return
	}
	curStart, _ := d.buf.GetLine(0)
	if curStart == prevStart {
		// this shouldn't happen
		panic("this should be unreachable: curStart == prevStart")
	}

	bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
	offsetCurLine := int(bufPos) - curStart

	rowWidth := prevEnd - prevStart
	if offsetCurLine > rowWidth {
		offsetCurLine = rowWidth
	}

	d.buf.Seek(int64(prevStart+offsetCurLine), io.SeekStart)

	d.cursorCoord.Y--
	d.cursorCoord.X = offsetCurLine
	d.redrawCursor()
}

func (d *DisplayBox) MvDown() {
	d.cursorPosSanityCheck()

	nextStart, nextEnd := d.buf.GetLine(1)
	if nextStart == -1 {
		// on last line
		return
	}

	curStart, _ := d.buf.GetLine(0)

	bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
	offsetCurLine := int(bufPos) - curStart

	rowWidth := nextEnd - nextStart

	if offsetCurLine > rowWidth {
		offsetCurLine = rowWidth
	}

	_, err := d.buf.Seek(int64(nextStart+offsetCurLine), io.SeekStart)
	if err != nil {
		panic(fmt.Sprintf("MvDown unexpected seek error: %s", err))
	}

	d.cursorCoord.Y++
	d.cursorCoord.X = offsetCurLine
	d.redrawCursor()
}

func (d *DisplayBox) MvBOL() {
	d.cursorPosSanityCheck()

	lineStart, _ := d.buf.GetLine(0)

	d.buf.Seek(int64(lineStart), io.SeekStart)
	d.cursorCoord.X = 0
	d.redrawCursor()
}

func (d *DisplayBox) MvEOL() {
	d.cursorPosSanityCheck()

	lineStart, lineEnd := d.buf.GetLine(0)
	endBufPos := d.buf.Size()

	if lineEnd == endBufPos-1 {
		lineEnd = endBufPos
	}

	d.buf.Seek(int64(lineEnd), io.SeekStart)

	d.cursorCoord.X = lineEnd - lineStart
	d.redrawCursor()
}

func (d *DisplayBox) redrawCursor() {
	tc := d.viewPortToTermCoord(d.cursorCoord)
	d.vt100.MoveToCoord(tc)
	d.cursorPosSanityCheck()
}

func (d *DisplayBox) InsertNewline() {
	d.cursorPosSanityCheck()
	var (
		haveSpaceBelow = d.firstRowT+d.termOwnedRows <= d.termSize.Row
		haveSpaceAbove = d.firstRowT > 1
	)

	d.buf.Insert([]byte{'\n'})

	if haveSpaceBelow {
		// we can grow downward
		d.editableRows++
		d.termOwnedRows++

		d.cursorCoord.X = 0
		d.cursorCoord.Y++
		d.Redraw()
	} else if haveSpaceAbove {
		// we can trigger a scroll to grow upwards

		d.vt100.Write([]byte("\r\n")) // trigger a line scroll
		d.editableRows++
		d.termOwnedRows++
		d.firstRowT--

		d.cursorCoord.X = 0

		d.Redraw()
	} else {
		// we are at max displaybox
		// we need to internally scroll
		panic("internal scroll not implemented yet")
	}
}

func (d *DisplayBox) Insert(b []byte) {
	d.cursorPosSanityCheck()
	d.buf.Insert(b)
	d.cursorCoord.X += len(b)

	d.redrawLine()
}

// Delete character under cursor
func (d *DisplayBox) Del() {
	d.cursorPosSanityCheck()

	startCoord := *d.cursorCoord
	d.MvRight()

	if startCoord == *d.cursorCoord {
		// at EOL or EOB, don't do anything
		return
	}

	d.Backspace()
}

// Delete previous character
func (d *DisplayBox) Backspace() {
	d.cursorPosSanityCheck()

	deleted := d.buf.Delete(1)

	if len(deleted) < 1 {
		return
	}

	if deleted[0] == '\n' {
		// we've deleted the previous newline. We need to redraw the previous lines and all following lines

		lineStart, _ := d.buf.GetLine(0)
		bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
		lineOffset := bufPos - int64(lineStart)
		d.cursorCoord.Y--

		d.cursorCoord.X = int(lineOffset)
		d.Redraw()
		return
	}

	d.cursorCoord.X--
	d.redrawLine()
}

func (d *DisplayBox) Redraw() {
	row := d.firstRowT

	d.vt100.MoveTo(row, 1)
	for i := 0; i < d.boarderTop; i++ {
		d.vt100.Write([]byte("~~~~"))
		row++
		d.vt100.MoveTo(row, 1)
	}

	for i := 0; i < d.editableRows; i++ {
		d.vt100.ClearToEndOfLine()

		offset := i - d.cursorCoord.Y
		bufStartLine, bufEndLine := d.buf.GetLine(offset)
		if bufStartLine == -1 {
			break
		}
		line := make([]byte, bufEndLine-bufStartLine)
		d.buf.ReadAt(line, int64(bufStartLine))

		for j := 0; j < d.boarderLeft; j++ {
			d.vt100.Write([]byte("~"))
		}

		d.vt100.Write(line)

		if d.boarderRight > 0 {
			if len(line) < d.termSize.Col+d.boarderLeft+d.boarderRight {
				for i := d.boarderLeft + len(line); i < d.termSize.Col-1; i++ {
					d.vt100.Write([]byte(" "))
				}
				d.vt100.Write([]byte("~"))
			}
		}

		row++
		d.vt100.MoveTo(row, 1)
	}

	for i := 0; i < d.boarderBottom; i++ {
		d.vt100.Write([]byte("~~~~"))
		row++
		if i < d.boarderBottom-1 {
			d.vt100.MoveTo(row, 1)
		}
	}

	d.redrawCursor()
}

type viewPortCoord struct {
	X, Y int
}

func (d *DisplayBox) cursorPosSanityCheck() {

	startLine, _ := d.buf.GetLine(0)
	bufOffset, _ := d.buf.Seek(0, io.SeekCurrent)

	lineOffset := int(bufOffset) - startLine
	if lineOffset != d.cursorCoord.X {
		panic(fmt.Sprintf("cursor pos out of sync with buf: cursorX=%d buf=%d", d.cursorCoord.X, lineOffset))
	}

	calced := d.viewPortToTermCoord(d.cursorCoord)

	actualPos := d.vt100.CursorPos()
	if actualPos != calced {
		panic(fmt.Sprintf("cursor pos out of sync! expected:%+v but was:%+v", calced, actualPos))
	}

	if d.cursorCoord.Y >= d.editableRows {
		panic(fmt.Sprintf("cursor pos >= editableRows posX=%d rowCount=%d", d.cursorCoord.Y, d.editableRows))
	}
}

func (d *DisplayBox) viewPortToTermCoord(vp *viewPortCoord) vt100.TermCoord {
	colT := vp.X + d.boarderLeft + 1
	rowT := vp.Y + d.firstRowT + d.boarderTop

	return vt100.TermCoord{
		Col: colT,
		Row: rowT,
	}
}

func (d *DisplayBox) redrawLine() {
	tc := d.viewPortToTermCoord(d.cursorCoord)
	d.vt100.MoveTo(tc.Row, 1)
	d.vt100.ClearToEndOfLine()

	lineStart, lineEnd := d.buf.GetLine(0)
	lineBuf := make([]byte, lineEnd-lineStart+1)
	d.buf.ReadAt(lineBuf, int64(lineStart))

	for i := 0; i < d.boarderLeft; i++ {
		d.vt100.Write([]byte("~"))
	}

	lineBuf = bytes.TrimRight(lineBuf, "\r\n")
	d.vt100.Write(lineBuf)

	if d.boarderRight > 0 {
		if len(lineBuf) < d.termSize.Col+d.boarderLeft+d.boarderRight {
			for i := d.boarderLeft + len(lineBuf); i < d.termSize.Col-1; i++ {
				d.vt100.Write([]byte(" "))
			}
			d.vt100.Write([]byte("~"))
		}
	}

	d.redrawCursor()
}

func (d *DisplayBox) DebugInfo() string {

	startLine, _ := d.buf.GetLine(0)
	bufOffset, _ := d.buf.Seek(0, io.SeekCurrent)
	lineOffset := int(bufOffset) - startLine

	return fmt.Sprintf(`DisplayBox:
editableRows=%d
termOwnedRows=%d
firstRowT=%d
bufOffsetX=%d
cursorX=%d
cursorY=%d`, d.editableRows, d.termOwnedRows, d.firstRowT, lineOffset, d.cursorCoord.X, d.cursorCoord.Y)

}
