package displaybox

import (
	"fmt"

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
}

func (d *DisplayBox) MvRight() {
}

func (d *DisplayBox) MvUp() {
}

func (d *DisplayBox) MvDown() {
}

func (d *DisplayBox) MvBOL() {
}

func (d *DisplayBox) MvEOL() {
}

func (d *DisplayBox) InsertNewline() {
	var (
		haveSpaceBelow = d.firstRowT+d.termOwnedRows < d.termSize.Row
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
	cursorT := d.cursorPosTermSpace()
	d.buf.Insert(b)
	d.cursorCoord.X++

	d.redrawLine(cursorT)
}

func (d *DisplayBox) Delete() {
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

	desiredCursorLocT := d.viewPortToTermCoord(d.cursorCoord)
	d.vt100.MoveToCoord(desiredCursorLocT)
}

type viewPortCoord struct {
	X, Y int
}

// returns the current cursor location in terminal coordinates
func (d *DisplayBox) cursorPosTermSpace() vt100.TermCoord {
	calced := d.viewPortToTermCoord(d.cursorCoord)

	actualPos := d.vt100.CursorPos()
	if actualPos != calced {
		panic(fmt.Sprintf("cursor pos out of sync! expected:%+v but was:%+v", calced, actualPos))
	}

	return actualPos
}

func (d *DisplayBox) viewPortToTermCoord(vp *viewPortCoord) vt100.TermCoord {
	colT := vp.X + d.boarderLeft + 1
	rowT := vp.Y + d.firstRowT + d.boarderTop

	return vt100.TermCoord{
		Col: colT,
		Row: rowT,
	}
}

func (d *DisplayBox) redrawLine(cursorT vt100.TermCoord) {
	d.vt100.MoveTo(cursorT.Row, 1)
	d.vt100.ClearToEndOfLine()

	lineStart, lineEnd := d.buf.GetLine(0)
	lineBuf := make([]byte, lineEnd-lineStart+1)
	d.buf.ReadAt(lineBuf, int64(lineStart))

	for i := 0; i < d.boarderLeft; i++ {
		d.vt100.Write([]byte("~"))
	}

	d.vt100.Write(lineBuf)

	if d.boarderRight > 0 {
		if len(lineBuf) < d.termSize.Col+d.boarderLeft+d.boarderRight {
			for i := d.boarderLeft + len(lineBuf); i < d.termSize.Col-1; i++ {
				d.vt100.Write([]byte(" "))
			}
			d.vt100.Write([]byte("~"))
		}
	}

	desiredCursorLocT := d.viewPortToTermCoord(d.cursorCoord)
	d.vt100.MoveToCoord(desiredCursorLocT)
}
