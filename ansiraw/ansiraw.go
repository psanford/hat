package ansiraw

import "bytes"

func ParseRaw(raw []byte) RawEvent {

	var (
		pageUp   = []byte{ESC, '[', '5', '~'}
		pageDown = []byte{ESC, '[', '6', '~'}
	)

	if bytes.Equal(raw, pageUp) {
		return PageUp
	} else if bytes.Equal(raw, pageDown) {
		return PageDown
	}

	return Unknown
}

const ESC = 0x1B

type RawEvent string

const (
	Unknown  RawEvent = "unknown"
	PageDown RawEvent = "page_down"
	PageUp   RawEvent = "page_up"
)
