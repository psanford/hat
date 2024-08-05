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

	vt100        *vt100.VT100
	borderTop    int
	borderLeft   int
	borderRight  int
	borderBottom int
	addBorder    bool
	buf          *gapbuffer.GapBuffer

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
		d.borderTop = 1
		d.borderLeft = 1
		d.borderRight = 1
		d.borderBottom = 1
		d.termOwnedRows = 3

		if d.firstRowT > d.termSize.Row-2 {
			// if we are at the bottom, we need to account for inserting the borders
			// in firstRowT
			for i := d.firstRowT; i > d.termSize.Row-2; i-- {
				d.firstRowT--
			}
		}

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

	startPos, _ := d.buf.GetLine(0)
	curPos, _ := d.buf.Seek(0, io.SeekCurrent)
	if curPos == int64(startPos) {
		return
	}
	d.buf.Seek(-1, io.SeekCurrent)

	if d.cursorCoord.X > 0 {
		d.cursorCoord.X--
		d.redrawCursor()
	} else {
		// scroll line left
		d.redrawLine()
	}
}

func (d *DisplayBox) MvRight() {
	d.cursorPosSanityCheck()

	bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
	endBufPos := d.buf.Size()

	// if we're at the very end of the file our cursor should be at lastpos+1
	_, eolPos := d.buf.GetLine(0)

	if eolPos == endBufPos-1 {
		lastChar := make([]byte, 1)
		d.buf.ReadAt(lastChar, int64(endBufPos)-1)
		if lastChar[0] != '\n' {
			eolPos = endBufPos
		}
	}
	if bufPos == int64(eolPos) {
		return
	}

	_, err := d.buf.Seek(1, io.SeekCurrent)
	if err != nil {
		panic(fmt.Sprintf("MvRight seek forward unexepected error: %s", err))
	}

	if d.cursorCoord.X < d.viewPortWidth()-1 {
		d.cursorCoord.X++
		d.redrawCursor()
	} else {
		// scroll line right
		d.redrawLine()
	}
}

func (d *DisplayBox) viewPortWidth() int {
	return d.termSize.Col - d.borderLeft - d.borderRight
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

	d.cursorCoord.X = offsetCurLine
	if d.cursorCoord.Y > 0 {
		d.cursorCoord.Y--
		d.redrawCursor()
	} else {
		d.Redraw()
	}
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

	d.cursorCoord.X = offsetCurLine
	if d.cursorCoord.Y < d.editableRows-1 {
		d.cursorCoord.Y++
		d.redrawCursor()
	} else {
		d.Redraw()
	}
}

func (d *DisplayBox) MvBOL() {
	d.cursorPosSanityCheck()

	lineStart, _ := d.buf.GetLine(0)

	d.buf.Seek(int64(lineStart), io.SeekStart)
	d.cursorCoord.X = 0
	d.redrawLine()
}

func (d *DisplayBox) MvEOL() {
	d.cursorPosSanityCheck()

	lineStart, lineEnd := d.buf.GetLine(0)
	endBufPos := d.buf.Size()

	bufPos, _ := d.buf.Seek(0, io.SeekCurrent)

	if bufPos == int64(lineEnd) {
		return
	}

	if lineEnd == endBufPos-1 {
		lastChar := make([]byte, 1)
		d.buf.ReadAt(lastChar, int64(endBufPos)-1)
		if lastChar[0] != '\n' {
			lineEnd = endBufPos
		}
	}

	d.buf.Seek(int64(lineEnd), io.SeekStart)

	d.cursorCoord.X = lineEnd - lineStart
	if d.cursorCoord.X >= d.viewPortWidth() {
		d.cursorCoord.X = d.viewPortWidth() - 1
	}
	d.redrawLine()
}

// Returns the last owned row in terminal coordinate space
func (d *DisplayBox) LastOwnedRow() vt100.TermCoord {
	lastLine := d.firstRowT + d.termOwnedRows

	return vt100.TermCoord{
		Row: lastLine,
	}
}

func (d *DisplayBox) redrawCursor() {
	tc := d.viewPortToTermCoord(d.cursorCoord)
	d.vt100.MoveToCoord(tc)
	d.cursorPosSanityCheck()
}

func (d *DisplayBox) InsertNewline() {
	d.cursorPosSanityCheck()
	var (
		haveSpaceBelow      = d.firstRowT+d.termOwnedRows <= d.termSize.Row
		haveSpaceAbove      = d.firstRowT > 1
		editableRowsForward = d.editableRows - d.cursorCoord.Y - 1

		hasUnusedEitableRow bool
	)

	if editableRowsForward > 0 {
		startPos, _ := d.buf.GetLine(editableRowsForward)
		if startPos == -1 {
			hasUnusedEitableRow = true
		}
	}

	d.buf.Insert([]byte{'\n'})

	if hasUnusedEitableRow {
		d.cursorCoord.X = 0
		d.cursorCoord.Y++
		d.Redraw()
	} else if haveSpaceBelow {
		// we can grow downward
		d.editableRows++
		d.termOwnedRows++

		d.cursorCoord.X = 0
		d.cursorCoord.Y++
		d.Redraw()
	} else if haveSpaceAbove {
		// we can trigger a scroll to grow upwards

		d.vt100.ScrollUp()
		d.editableRows++
		d.termOwnedRows++
		d.firstRowT--

		d.cursorCoord.X = 0
		d.cursorCoord.Y++

		d.Redraw()
	} else {
		d.cursorCoord.X = 0
		d.Redraw()
	}
}

func (d *DisplayBox) Insert(p []byte) {
	// PMS: this is not correct for unicode characters
	// we probably should also check that b is printable

	d.cursorPosSanityCheck()

	for _, b := range p {
		if b == '\n' {
			d.InsertNewline()
		} else {
			d.buf.Insert([]byte{b})
			if d.cursorCoord.X < d.viewPortWidth()-1 {
				d.cursorCoord.X++
			}

			d.redrawLine()
		}
	}
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
	if d.borderTop > 0 {
		// draw borderTop
		var borderTop = defaultBorderTop

		startPos, _ := d.buf.GetLine((-1 * d.cursorCoord.Y) - 1)
		if startPos != -1 {
			// there's more rows above the top of the terminal, indicate that
			borderTop = overflowBorderTop
		}

		row := d.firstRowT
		d.vt100.MoveTo(row, 1)
		for i := 0; i < d.borderTop; i++ {
			d.vt100.ClearToEndOfLine()
			d.vt100.Write(borderTop)
			row++
			d.vt100.MoveTo(row, 1)
		}
	}

	for i := 0; i < d.editableRows; i++ {
		coord := viewPortCoord{X: 0, Y: i}
		d.redrawLineX(&coord)
	}

	if d.borderBottom > 0 {
		// draw borderBottom
		var borderBottom = defaultBorderBottom

		editableRowsForward := d.editableRows - d.cursorCoord.Y

		if editableRowsForward > 0 {
			startPos, _ := d.buf.GetLine(editableRowsForward)
			if startPos != -1 {
				// there's more rows above below terminal, indicate that
				borderBottom = overflowBorderBottom
			}
		}

		row := d.firstRowT + d.editableRows + 1
		d.vt100.MoveTo(row, 1)

		for i := 0; i < d.borderBottom; i++ {
			d.vt100.ClearToEndOfLine()
			d.vt100.Write(borderBottom)
			row++
			if i < d.borderBottom-1 {
				d.vt100.MoveTo(row, 1)
			}
		}
	}

	d.redrawCursor()
}

func (d *DisplayBox) TerminalResize() {
	newSize := d.vt100.Size()

	// Our source of truth for our location is bufPos
	// d.buf.Seek(0, io.SeekCurrent)

	oldSize := d.termSize
	d.termSize = newSize

	didShrink := func(oldSize, newSize int) bool {
		return oldSize > newSize
	}

	didGrow := func(oldSize, newSize int) bool {
		return oldSize < newSize
	}

	if didGrow(oldSize.Row, newSize.Row) {
		newRows := newSize.Row - oldSize.Row
		d.termOwnedRows += newRows
		d.editableRows += newRows
	} else if didShrink(oldSize.Row, newSize.Row) {
		// if we have unused space below us, consume that before adjusting anything

		shrinkAmt := oldSize.Row - newSize.Row

		spaceBelow := oldSize.Row - (d.firstRowT + d.termOwnedRows - 1)
		for spaceBelow > 0 && shrinkAmt > 0 {
			// there was some unused rows below us, consume them first
			spaceBelow--
			shrinkAmt--
		}

		if shrinkAmt > 0 {
			for d.firstRowT > 1 && shrinkAmt > 0 {
				// there's some space above us, take it
				d.vt100.ScrollUp()
				d.firstRowT--
				shrinkAmt--
			}

			if shrinkAmt > 0 {

				d.termOwnedRows -= shrinkAmt
				if d.termOwnedRows < 1+d.borderTop+d.borderBottom {
					stealAmt := 1 + d.borderTop + d.borderBottom - d.termOwnedRows
					d.firstRowT -= stealAmt
					if d.firstRowT < 1 {
						panic(fmt.Sprintf("Terminal too small: shrinkAmt=%d firstRow=%d", shrinkAmt, d.firstRowT))
					}
				}
				d.editableRows -= shrinkAmt
				if d.editableRows < 1 {

					for d.firstRowT != 1 {
						d.firstRowT--
						d.editableRows++
						if d.editableRows > 1 {
							break
						}
					}
					if d.editableRows < 1 {
						panic(fmt.Sprintf("Terminal too small: shrinkAmt=%d editableRows=%d", shrinkAmt, d.editableRows))
					}
				}

				d.cursorCoord.Y -= shrinkAmt
				if d.cursorCoord.Y < 0 {
					d.cursorCoord.Y = 0
				}
			}
		}

	}

	if didShrink(oldSize.Col, newSize.Col) {
		shrinkAmt := oldSize.Col - newSize.Col

		// our cursor is now out of bounds
		if d.cursorCoord.X >= d.viewPortWidth() {
			d.cursorCoord.X = d.cursorCoord.X - shrinkAmt
			if d.cursorCoord.X < 0 {
				d.cursorCoord.X = 0
			}
		}

	}

	d.Redraw()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type viewPortCoord struct {
	X, Y int
}

func (d *DisplayBox) cursorPosSanityCheck() {

	startLine, endLine := d.buf.GetLine(0)
	bufOffset, _ := d.buf.Seek(0, io.SeekCurrent)

	lineSize := endLine - startLine

	lineOffset := int(bufOffset) - startLine
	if lineOffset != d.cursorCoord.X && lineSize < d.viewPortWidth()-1 {
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
	colT := vp.X + d.borderLeft + 1
	rowT := vp.Y + d.firstRowT + d.borderTop

	return vt100.TermCoord{
		Col: colT,
		Row: rowT,
	}
}

func (d *DisplayBox) redrawLine() {
	d.redrawLineX(d.cursorCoord)
	d.redrawCursor()
}

var (
	defaultBorderTop    = []byte("~~~~")
	defaultBorderBottom = []byte("~~~~")
	defaultBorderLeft   = []byte("~")
	defaultBorderRight  = []byte("~")

	overflowBorderTop    = []byte("▲▲▲▲")
	overflowBorderBottom = []byte("▼▼▼▼")
	overflowBorderLeft   = []byte("◀")
	overflowBorderRight  = []byte("▶")
)

func (d *DisplayBox) redrawLineX(coord *viewPortCoord) {
	bufOffset := coord.Y - d.cursorCoord.Y

	tc := d.viewPortToTermCoord(coord)

	d.vt100.MoveTo(tc.Row, 1)
	d.vt100.ClearToEndOfLine()

	lineStart, lineEnd := d.buf.GetLine(bufOffset)
	if lineStart == -1 {
		if d.borderBottom > 0 {
			d.vt100.Write(defaultBorderBottom)
		}
		return
	}

	lineBuf := make([]byte, lineEnd-lineStart+1)
	i, _ := d.buf.ReadAt(lineBuf, int64(lineStart))
	lineBuf = lineBuf[:i]
	lineBuf = bytes.TrimRight(lineBuf, "\r\n")

	leftBorder := defaultBorderLeft
	rightBorder := defaultBorderRight

	if len(lineBuf) >= d.viewPortWidth()-1 {
		// our line is longer than the viewport

		var startVisible int
		if bufOffset == 0 {
			// If we are redrawing the line our cursor is on,
			// figure out the amount we need to scroll.
			// If we are on a different line we don't scroll
			cursorLineStart, _ := d.buf.GetLine(0)
			bufPos, _ := d.buf.Seek(0, io.SeekCurrent)
			posInLine := bufPos - int64(cursorLineStart)

			startVisible = int(posInLine) - d.cursorCoord.X
			if startVisible > 0 {
				leftBorder = overflowBorderLeft
			}
		}

		lineBuf = lineBuf[startVisible:]
		if len(lineBuf) > d.viewPortWidth()-1 {
			lineBuf = lineBuf[:d.viewPortWidth()-1]
			rightBorder = overflowBorderRight
		}
	}

	for i := 0; i < d.borderLeft; i++ {
		d.vt100.Write(leftBorder)
	}

	d.vt100.Write(lineBuf)

	if d.borderRight > 0 {
		if len(lineBuf) < d.termSize.Col+d.borderLeft+d.borderRight {
			for i := d.borderLeft + len(lineBuf); i < d.termSize.Col-1; i++ {
				d.vt100.Write([]byte(" "))
			}
			d.vt100.Write(rightBorder)
		}
	}
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
