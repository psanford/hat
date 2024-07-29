package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/psanford/hat/ansiparser"
	"github.com/psanford/hat/displaybox"
	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/terminal"
	"github.com/psanford/hat/vt100"
)

var border = flag.Bool("border", false, "show border")

func main() {
	flag.Parse()

	term := terminal.NewTerm(int(os.Stdin.Fd()))

	ed := newEditor(os.Stdin, os.Stdout, os.Stderr, term)

	ctx := context.Background()
	ed.run(ctx)
}

type editor struct {
	term  terminal.Terminal
	vt100 *vt100.VT100
	buf   *gapbuffer.GapBuffer
	disp  *displaybox.DisplayBox

	// line the command executable is on
	// e.g. $ hat
	// 1 indexed
	// promptLine     int
	// editorRowCount int

	parser *ansiparser.Parser

	debugLog io.Writer

	testEventProcessedCh chan struct{}

	in io.Reader
	// out *os.File or io.Writer
	// err io.Writer

	err error
}

func newEditor(in io.Reader, out io.Writer, err io.Writer, term terminal.Terminal) *editor {

	vt := vt100.New(term)
	gb := gapbuffer.New(2)

	ed := &editor{
		in:    in,
		term:  term,
		vt100: vt,
		buf:   gb,
	}

	return ed
}

func (ed *editor) run(ctx context.Context) {
	ed.term.EnableRawMode()
	defer ed.term.Restore()

	debug, _ := os.Create("/tmp/hat.debug.log")
	ed.debugLog = debug
	// ed.buf.Debug = debug

	ed.disp = displaybox.New(ed.vt100, ed.buf, *border)

	prevTermCols, prevTermRows := ed.term.Size()

	ed.parser = ansiparser.New(context.Background())
	ed.parser.SetLogger(func(s string, i ...interface{}) {
		fmt.Fprintf(ed.debugLog, s, i...)
		fmt.Fprintln(ed.debugLog)
	})
	events := ed.parser.EventChan()

MAIN_LOOP:
	for {
		termCols, termRows := ed.term.Size()
		if prevTermCols != termCols || prevTermRows != termRows {
			fmt.Fprintf(debug, "terminal resize! oldterm:<%d, %d> newterm:<%d, %d>\n", prevTermCols, prevTermRows, termCols, termRows)
			// XXXXXX
			// handle terminal resize here

			prevTermCols, prevTermRows = termCols, termRows
		}

		ed.writeDebugTerminalState()
		fmt.Fprintln(debug, ed.disp.DebugInfo())
		ed.readBytes()

		for {
			var e ansiparser.Event
			select {
			case e = <-events:
				fmt.Fprintf(debug, "event: %T %v\n", e, e)
			case <-ctx.Done():
				return
			default:
				fmt.Fprintf(debug, "\nCONTINUE MAIN_LOOP\n")

				continue MAIN_LOOP
			}

			switch ee := e.(type) {
			case ansiparser.Character:
				c := ee.Char
				if c == 0x7F { // ASCII DEL (backspace)
					ed.disp.Backspace()
				} else if c == '\r' {
					ed.disp.InsertNewline()
				} else if c == ctrlC || c == ctrlD {
					break MAIN_LOOP
				} else if c == ctrlA { // ctrl-a
					ed.disp.MvBOL()
				} else if c == ctrlE { // ctrl-e
					ed.disp.MvEOL()
				} else if c == ctrlL {
					// redraw the section of the terminal we own
					ed.disp.Redraw()
				} else {
					fmt.Fprintf(debug, "loop: is plain char <%c>\n", c)

					ed.disp.Insert([]byte{c})
				}
			case ansiparser.DeleteCharater:
				ed.disp.Del()
			case ansiparser.CursorMovement:
				switch ee.Direction {
				case ansiparser.Up:
					ed.disp.MvUp()
				case ansiparser.Down:
					ed.disp.MvDown()
				case ansiparser.Forward:
					ed.disp.MvRight()
				case ansiparser.Backward:
					ed.disp.MvLeft()
				}
			default:
				fmt.Fprintf(debug, "unhandled event: %T %v\n", e, e)
			}

			info := ed.buf.DebugInfo()
			os.WriteFile("/tmp/hat.current.buffer", info.Bytes(), 0600)

			select {
			case ed.testEventProcessedCh <- struct{}{}:
			default:
			}
		}
	}

	f, err := os.Create("/tmp/hat.out")
	if err != nil {
		panic(err)
	}
	ed.buf.Seek(0, io.SeekStart)
	_, err = io.Copy(f, ed.buf)
	if err != nil {
		panic(err)
	}
	f.Close()
}

func (ed *editor) writeDebugTerminalState() {
	termCols, termRows := ed.term.Size()
	coord := ed.vt100.CursorPos()

	f, err := os.Create("/tmp/hat.current.terminal")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Fprintln(f, strings.Repeat("@", termCols))
	fmt.Fprintf(f, "@@ w:%d h:%d cur_row:%d cur_col:%d @@\n", termCols, termRows, coord.Row, coord.Col)

	for row := 1; row <= termRows; row++ {
		bufRowOffset := row - coord.Row
		startLine, endLine := ed.buf.GetLine(bufRowOffset)

		if startLine < 0 && endLine < 0 {
			fmt.Fprintln(f, strings.Repeat("@", termCols))
			continue
		}

		lineText := make([]byte, endLine-startLine+1)
		ed.buf.ReadAt(lineText, int64(startLine))

		for col := 1; col <= termCols; col++ {
			if row == coord.Row && col == coord.Col {
				f.Write([]byte("_"))
				continue
			}

			if col <= len(lineText) {
				b := lineText[col-1]
				if b == '\n' {
					f.Write([]byte{'$'})
				} else {
					f.Write([]byte{b})
				}
			} else {
				f.Write([]byte{'~'})
			}
		}
		f.Write([]byte{'\n'})
	}
}

func (ed *editor) cursorPos() (row, col int) {
	coord := ed.vt100.CursorPos()
	return coord.Row, coord.Col
}

func (ed *editor) readChar() (byte, error) {
	if ed.err != nil {
		return 0, ed.err
	}
	b := make([]byte, 1)
	_, err := ed.in.Read(b)
	if err != nil {
		ed.err = err
	}

	_, err = ed.parser.Write(b)
	if err != nil {
		panic(err)
	}

	return b[0], err
}

func (ed *editor) readBytes() (int, error) {
	if ed.err != nil {
		return 0, ed.err
	}
	b := make([]byte, 128)
	n, err := ed.in.Read(b)
	if err != nil {
		ed.err = err
	}

	_, err = ed.parser.Write(b[:n])
	if err != nil {
		ed.err = err
	}

	return n, err
}

const (
	ctrlA = 0x01
	ctrlB = 0x02
	ctrlC = 0x03
	ctrlD = 0x04
	ctrlE = 0x05
	ctrlL = 0x0C
)

func ctrlKey(c byte) byte {
	return c & 0x1f
}

func bail(err error) {
	panic(err)
}
