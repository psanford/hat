package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/psanford/ansiterm"
	"github.com/psanford/hat/ansiraw"
	"github.com/psanford/hat/displaybox"
	"github.com/psanford/hat/gapbuffer"
	"github.com/psanford/hat/terminal"
	"github.com/psanford/hat/vt100"
)

var border = flag.Bool("border", false, "show border")
var debugLog = flag.Bool("debug", false, "write debug logs")

func main() {
	flag.Parse()

	out := os.Stdout

	if err := syscall.SetNonblock(0, true); err != nil {
		panic(err)
	}
	in := os.NewFile(0, "stdin")

	var srcFile *os.File

	args := flag.Args()
	if len(args) > 1 {
		log.Fatalf("Can only accept 1 file right now")
	}

	if len(args) == 1 {
		inF, err := os.Open(args[0])
		if err == nil {
			srcFile = inF
			defer in.Close()
		} else if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		out = nil
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0777)
	if err != nil {
		// we don't have a terminal, behave like cat
		_, err := io.Copy(out, in)
		if err != nil && err != io.EOF {
			panic(err)
		}
		return
	}

	term := terminal.NewTerm(int(tty.Fd()))

	ed := newEditor(in, srcFile, term)

	ctx := context.Background()
	save := ed.run(ctx)

	if !save {
		log.Println("abort")
		os.Exit(1)
	}

	ed.buf.Seek(0, io.SeekStart)

	if len(args) == 1 {
		out, err = os.Create(args[0])
		if err != nil {
			log.Fatal(err)
		}
	}

	if terminal.IsTerminal(int(out.Fd())) {
		// stdout is a terminal, don't re-echo the output
		return
	}

	io.Copy(out, ed.buf)
}

type editor struct {
	term  terminal.Terminal
	vt100 *vt100.VT100
	buf   *gapbuffer.GapBuffer
	disp  *displaybox.DisplayBox

	parser *ansiterm.AnsiParser

	debugLog io.Writer

	testEventProcessedCh chan struct{}

	in       *os.File
	inReader io.Reader
	srcFile  *os.File
}

func newEditor(in, srcFile *os.File, term terminal.Terminal) *editor {
	vt := vt100.New(term)
	gb := gapbuffer.New(2)

	ed := &editor{
		in:       in,
		inReader: in,
		srcFile:  srcFile,
		term:     term,
		vt100:    vt,
		buf:      gb,
	}

	return ed
}

func (ed *editor) run(parentCtx context.Context) (save bool) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	save = true
	ed.term.EnableRawMode()
	defer ed.term.Restore()

	if *debugLog {
		debug, _ := os.Create("/tmp/hat.debug.log")
		ed.debugLog = debug
	}

	sigChan := make(chan os.Signal, 1)
	resizeChan := make(chan struct{}, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGWINCH)
	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				save = false
				cancel()
				return
			case syscall.SIGWINCH:
				select {
				case resizeChan <- struct{}{}:
				default:
				}
			}
		}
	}()

	cursorT, extraBytes, err := ed.vt100.CursorPos()
	if err != nil {
		log.Fatalf("Failed to read cursor position: %s", err)
	}

	if len(extraBytes) > 0 {
		extraReader := bytes.NewReader(extraBytes)
		ed.inReader = io.MultiReader(extraReader, ed.inReader)
	}

	if cursorT.Col != 1 {
		// There's some existing content on the line we are currently on,
		// move to the next line.
		// Git does this, for example.
		ed.vt100.Write([]byte("\r\n"))
	}

	ed.disp = displaybox.New(ed.vt100, ed.buf, *border, *cursorT)

	defer func() {
		// mv cursor to bottom of our controlled area so we don't mess up
		// the terminal
		tc := ed.disp.LastOwnedRow()
		ed.vt100.MoveToCoord(tc)
	}()

	if ed.srcFile != nil {
		buf := make([]byte, 128)
		for {
			n, err := ed.srcFile.Read(buf)
			if n > 0 {
				text := buf[:n]
				ed.disp.Insert(text)
			}
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}
		}
	}

	eventChan := make(chan ansiterm.AnsiEvent, 10)
	var opts []ansiterm.Option

	if *debugLog {
		opt := ansiterm.WithLogf(func(s string, i ...interface{}) {
			fmt.Fprintf(ed.debugLog, s, i...)
			fmt.Fprintln(ed.debugLog)
		})
		opts = append(opts, opt)
	}

	ed.parser = ansiterm.CreateParser(eventChan, opts...)

MAIN_LOOP:
	for {
		ed.debugPrintf("%s\n", ed.disp.DebugInfo())

		readResultChan := make(chan readResult)
		go func() {
			result := ed.readBytes()
			if errors.Is(result.err, os.ErrDeadlineExceeded) {
				ed.debugPrintf("read input canceled\n")
			}
			readResultChan <- result
		}()

		select {
		case result := <-readResultChan:
			if result.err != nil {
				if result.err == io.EOF {
					return
				}
				log.Fatalf("read err: %s", result.err)
				continue MAIN_LOOP
			}
		case <-resizeChan:
			ed.debugPrintf("got resize event\n")
			ed.in.SetReadDeadline(time.Now().Add(-time.Microsecond))
			<-readResultChan
			ed.in.SetReadDeadline(time.Time{})
			ed.disp.TerminalResize()
			continue
		case <-ctx.Done():
			return
		}

		for {
			var e ansiterm.AnsiEvent
			select {
			case e = <-eventChan:
				ed.debugPrintf("event: %T %v\n", e, e)
			case <-ctx.Done():
				return
			default:
				ed.debugPrintf("\nCONTINUE MAIN_LOOP\n")
				continue MAIN_LOOP
			}

			switch ee := e.(type) {
			case *ansiterm.Print:
				ed.debugPrintf("loop: is plain char <%s>\n", ee.B)

				p := ee.B
				for len(p) > 0 {
					r, size := utf8.DecodeRune(p)
					p = p[size:]
					if r == 0x7F {
						ed.disp.Backspace()
					} else {
						ed.disp.Insert([]byte(string(r)))
					}
				}
			case *ansiterm.Execute:
				for _, c := range ee.B {
					if c == 0x7F { // ASCII DEL (backspace)
						ed.disp.Backspace()
					} else if c == '\r' {
						ed.disp.InsertNewline()
					} else if c == ctrlD {
						break MAIN_LOOP
					} else if c == ctrlA { // ctrl-a
						ed.disp.MvBOL()
					} else if c == ctrlE { // ctrl-e
						ed.disp.MvEOL()
					} else if c == ctrlL {
						// redraw the section of the terminal we own
						ed.disp.Redraw()
					} else {
						ed.debugPrintf("unsupported control char<%c>\n", c)
					}
				}
			case *ansiterm.CursorUp:
				for i := 0; i < ee.N; i++ {
					ed.disp.MvUp()
				}
			case *ansiterm.CursorDown:
				for i := 0; i < ee.N; i++ {
					ed.disp.MvDown()
				}
			case *ansiterm.CursorForward:
				for i := 0; i < ee.N; i++ {
					ed.disp.MvRight()
				}
			case *ansiterm.CursorBackward:
				for i := 0; i < ee.N; i++ {
					ed.disp.MvLeft()
				}
			case *ansiterm.DeleteCharacter:
				for i := 0; i < ee.N; i++ {
					ed.disp.Del()
				}
			// case *CursorNextLine:
			// case *CursorPreviousLine:
			// case *CursorHorizontalAbsolute:
			// case *VerticalLinePositionAbsolute:
			// case *CursorPosition:
			// case *HorizontalVerticalPosition:
			// case *TextCursorEnableMode:
			// case *OriginMode:
			// case *ColumnMode:
			// case *EraseInDisplay:
			// case *EraseInLine:
			// case *InsertLine:
			// case *DeleteLine:
			// case *InsertCharacter:
			// case *SetGraphicsRendition:
			// case *ScrollUp:
			// case *ScrollDown:
			// case *DeviceAttributes:
			// case *SetTopAndBottomMargins:
			// case *Index:
			// case *ReverseIndex:
			default:
				switch ansiraw.ParseRaw(e.Raw()) {
				case ansiraw.PageDown:
					ed.disp.MvPgDown()
				case ansiraw.PageUp:
					ed.disp.MvPgUp()
				default:
					ed.debugPrintf("Unhandled event type: %T %+v\n", ee, ee)
				}
			}

			if *debugLog {
				info := ed.buf.DebugInfo()
				os.WriteFile("/tmp/hat.current.buffer", info.Bytes(), 0600)
			}

			select {
			case ed.testEventProcessedCh <- struct{}{}:
			default:
			}
		}
	}

	return
}

func (ed *editor) debugPrintf(format string, args ...any) {
	if ed.debugLog != nil {
		fmt.Fprintf(ed.debugLog, format, args...)
	}
}

func (ed *editor) readBytes() readResult {
	var result readResult

	var (
		n     int
		err   error
		total int

		b        = make([]byte, 128)
		readMore = true
	)

	for readMore {
		n, err = ed.inReader.Read(b[total:])

		if n > 0 {
			total += n
		}
		if len(b[total:]) == n {
			newBuf := make([]byte, len(b)*2)
			copy(newBuf, b)
			b = newBuf
		} else {
			readMore = false
		}

		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}

		if err != nil {
			result.err = err
			break
		}
	}

	if total > 0 {
		_, err = ed.parser.Parse(b[:total])
		if err != nil {
			result.err = err
		}

	}
	result.n = total

	return result
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

type readResult struct {
	n   int
	err error
}
