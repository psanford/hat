package ansiparser

import (
	"context"

	"github.com/psanford/go-ansiterm"
)

func New(ctx context.Context) *Parser {
	eh := &eventHandler{
		events: make(chan Event, 128), // set to the same size as our read buffer
		stop:   make(chan struct{}),
	}

	go func() {
		<-ctx.Done()
		eh.err = ctx.Err()
		close(eh.stop)
	}()

	p := &Parser{
		eventHandler: eh,
		parser:       ansiterm.CreateParser("Ground", eh),
	}

	return p
}

type Parser struct {
	eventHandler *eventHandler
	parser       *ansiterm.AnsiParser
}

func (p *Parser) EventChan() chan Event {
	return p.eventHandler.events
}

func (p *Parser) Write(b []byte) (int, error) {
	return p.parser.Parse(b)
}

func (p *Parser) SetLogger(logf func(string, ...interface{})) {
	p.parser = ansiterm.CreateParser("Ground", p.eventHandler, ansiterm.WithLogf(logf))
}

type Event interface {
}

// Normal Character
type Character struct {
	Char byte
}

type CursorMovement struct {
	Direction Direction
	Amount    int
}

type OrginMode struct {
	RestrictToMargin bool
}

type ColumnMode132 struct {
	Mode132 bool // true == 132 column mode; false == 80 column mode
}

type EraseInDisplay struct {
	Mode EraseMode
}

type EraseInLine struct {
	Mode EraseMode
}

type InsertLine struct {
	Count int
}

type DeleteLine struct {
	Count int
}

type InsertCharater struct {
	Count int
}

type DeleteCharater struct {
	Count int
}

type Direction int

const (
	Up Direction = iota + 1
	Down
	Forward
	Backward
	BeginningOfNextLine
	BeginningOfPrevLine
	Column
	Row
)

type EraseMode int

const (
	EraseFromCursorToEnd       EraseMode = 0
	EraseFromBeginningToCursor EraseMode = 1
	EraseAll                   EraseMode = 2
)

type CursorPosition struct {
	Row int // 1 indexed
	Col int // 1 indexed
}

type CursorVisibility struct {
	Visible bool
}

type eventHandler struct {
	events chan Event
	stop   chan struct{}
	err    error
}

func (h *eventHandler) sendEvent(e Event) error {
	select {
	case h.events <- e:
	case <-h.stop:
		return h.err
	}
	return nil
}

// Print
func (h *eventHandler) Print(b byte) error {
	e := Character{Char: b}
	return h.sendEvent(e)
}

// Execute C0 commands
func (h *eventHandler) Execute(b byte) error {
	e := Character{Char: b}
	return h.sendEvent(e)
}

// CUrsor Up
func (h *eventHandler) CUU(n int) error {
	e := CursorMovement{
		Direction: Up,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// CUrsor Down
func (h *eventHandler) CUD(n int) error {
	e := CursorMovement{
		Direction: Down,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// CUrsor Forward
func (h *eventHandler) CUF(n int) error {
	e := CursorMovement{
		Direction: Forward,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// CUrsor Backward
func (h *eventHandler) CUB(n int) error {
	e := CursorMovement{
		Direction: Backward,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// Cursor to Next Line
func (h *eventHandler) CNL(n int) error {
	e := CursorMovement{
		Direction: BeginningOfNextLine,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// Cursor to Previous Line
func (h *eventHandler) CPL(n int) error {
	e := CursorMovement{
		Direction: BeginningOfPrevLine,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// Cursor Horizontal position Absolute
func (h *eventHandler) CHA(n int) error {
	e := CursorMovement{
		Direction: Column,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// Vertical line Position Absolute
func (h *eventHandler) VPA(n int) error {
	e := CursorMovement{
		Direction: Row,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// CUrsor Position
func (h *eventHandler) CUP(row int, column int) error {
	e := CursorPosition{
		Row: row,
		Col: column,
	}
	return h.sendEvent(e)
}

// Horizontal and Vertical Position (depends on PUM)
func (h *eventHandler) HVP(row int, column int) error {
	e := CursorPosition{
		Row: row,
		Col: column,
	}
	return h.sendEvent(e)
}

// Text Cursor Enable Mode
func (h *eventHandler) DECTCEM(enable bool) error {
	// CSI ? P m h
	// P == 25
	// Show Cursor (DECTCEM)
	e := CursorVisibility{
		Visible: enable,
	}
	return h.sendEvent(e)
}

// Origin Mode
func (h *eventHandler) DECOM(set bool) error {
	// CSI ? P m h
	// P == 25
	// Origin Mode (DECOM)
	// When DECOM is set, the home cursor position is at the upper-left corner of the screen, within the margins. The starting point for line numbers depends on the current top margin setting. The cursor cannot move outside of the margins.
	// When DECOM is reset (false), the home cursor position is at the upper-left corner of the screen. The starting point for line numbers is independent of the margins. The cursor can move outside of the margins.
	e := OrginMode{
		RestrictToMargin: set,
	}
	return h.sendEvent(e)
}

// 132 Column Mode
func (h *eventHandler) DECCOLM(set bool) error {
	// CSI ? P m h
	// P = 3
	// 132 Column Mode (DECCOLM)
	e := ColumnMode132{
		Mode132: set,
	}
	return h.sendEvent(e)
}

// Erase in Display
func (h *eventHandler) ED(mode int) error {
	e := EraseInDisplay{
		Mode: EraseMode(mode),
	}
	return h.sendEvent(e)
}

// Erase in Line
func (h *eventHandler) EL(mode int) error {
	e := EraseInLine{
		Mode: EraseMode(mode),
	}
	return h.sendEvent(e)
}

// Insert Line
func (h *eventHandler) IL(count int) error {
	e := InsertLine{
		Count: count,
	}
	return h.sendEvent(e)
}

// Delete Line
func (h *eventHandler) DL(count int) error {
	e := DeleteLine{
		Count: count,
	}
	return h.sendEvent(e)
}

// Insert Character
func (h *eventHandler) ICH(count int) error {
	e := InsertCharater{
		Count: count,
	}
	return h.sendEvent(e)
}

// Delete Character
func (h *eventHandler) DCH(count int) error {
	e := DeleteCharater{
		Count: count,
	}
	return h.sendEvent(e)
}

type SgrAttribute int

const (
	SgrAllOff       SgrAttribute = 0
	SgrBold         SgrAttribute = 1
	SgrUnderline    SgrAttribute = 4
	SgrBlinking     SgrAttribute = 5
	SgrNegative     SgrAttribute = 7
	SgrInvisible    SgrAttribute = 8
	SgrBoldOff      SgrAttribute = 22
	SgrUnderlineOff SgrAttribute = 24
	SgrBlinkingOff  SgrAttribute = 25
	SgrNegativeOff  SgrAttribute = 27
	SgrInvisibleOff SgrAttribute = 28
)

type SetGraphicsRendition struct {
	Attributes []SgrAttribute
}

// Set Graphics Rendition
func (h *eventHandler) SGR(attrs []int) error {
	e := SetGraphicsRendition{
		Attributes: make([]SgrAttribute, len(attrs)),
	}
	for i, v := range attrs {
		e.Attributes[i] = SgrAttribute(v)
	}
	return h.sendEvent(e)
}

type PanTerminal struct {
	// only Up and Down are valid Pan directions
	Direction Direction
	Amount    int
}

// Pan Down
func (h *eventHandler) SU(n int) error {
	e := PanTerminal{
		Direction: Down,
		Amount:    n,
	}
	return h.sendEvent(e)
}

// Pan Up
func (h *eventHandler) SD(n int) error {
	e := PanTerminal{
		Direction: Up,
		Amount:    n,
	}
	return h.sendEvent(e)
}

type PageUp struct {
}

type PageDown struct {
}

// This is supposed to be Device Attributes, but we are
// abusing it for passing out other states from the parser
func (h *eventHandler) DA(args []string) error {
	if len(args) == 3 && args[0] == "CSI" && args[1] == "~" {
		switch args[2] {
		case "5":
			return h.sendEvent(PageUp{})
		case "6":
			return h.sendEvent(PageDown{})
		}
	}

	return nil
}

// Set Top and Bottom Margins
func (h *eventHandler) DECSTBM(int, int) error {
	panic("DECSTBM")
	return nil
}

// Index
func (h *eventHandler) IND() error {
	panic("IND")
	return nil
}

// Reverse Index
func (h *eventHandler) RI() error {
	panic("RI")
	return nil
}

// Flush updates from previous commands
func (h *eventHandler) Flush() error {
	return nil
}
