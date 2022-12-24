// gapbuffer implements the basic gapbuffer datastructure.
package gapbuffer

// gapbuffer basics:
// you have a front-segment, a back-segment and a gap inbetween
// your cursor is always at the beginning of the gap, so you can insert text where ever that is
// as you navigate your cursor around you copy data from one segment to the other depending on
// if you navigate forwards or backwards. This is relatively effecient since its a single memcopy
// for each move.

import (
	"errors"
	"fmt"
	"io"
)

type GapBuffer struct {
	buf       []byte
	frontSize int64
	backSize  int64

	// XXX remove
	Debug io.Writer
}

// New creates a new GapBuffer with an initial size.
func New(size int) *GapBuffer {
	if size < 2 {
		size = 1 << 16
	}

	return &GapBuffer{
		buf:       make([]byte, size),
		frontSize: 0,
		backSize:  0,
	}
}

func (b *GapBuffer) Seek(offset int64, whence int) (int64, error) {
	var abs int64

	totalSize := b.frontSize + b.backSize
	b.debugPrintf("gb: seek: %d %d (totalSize: %d)\n", offset, whence, totalSize)

	switch whence {
	case io.SeekCurrent:
		abs = b.frontSize + offset
	case io.SeekStart:
		abs = offset
	case io.SeekEnd:
		abs = b.frontSize + b.backSize - offset
	default:
		return 0, errors.New("GapBuffer.Seek: invalid whence")
	}

	if abs < 0 {
		return 0, errors.New("GapBuffer.Seek: negative position")
	}

	if abs > b.frontSize+b.backSize {
		abs = b.frontSize + b.backSize
	}

	b.debugPrintf("gb: moveCursor: %d\n", abs-b.frontSize)

	b.moveCursor(abs - b.frontSize)

	return abs, nil
}

func (b *GapBuffer) Insert(p []byte) (int, error) {
	if len(p) > b.gapSize() {
		b.grow(len(p))
	}

	copy(b.buf[b.frontSize:], p)
	b.frontSize += int64(len(p))

	return len(p), nil
}

func (b *GapBuffer) Delete(n int) {
	if n > int(b.frontSize) {
		n = int(b.frontSize)
	}

	b.frontSize -= int64(n)
}

func (b *GapBuffer) ReadAt(p []byte, off int64) (int, error) {
	tailAmt := 0
	tailOffset := 0
	var tailP []byte

	if tailOffset > int(b.frontSize+b.backSize) {
		return 0, io.EOF
	}

	if off < b.frontSize {
		if off+int64(len(p)) < b.frontSize {
			copy(p, b.buf[off:int(off)+len(p)])
			return len(p), nil
		} else {
			n := copy(p, b.buf[off:b.frontSize])
			tailAmt = len(p) - int(b.frontSize-off)
			tailP = p[n:]
		}
	} else {
		tailAmt = len(p)
		tailOffset = int(off - b.frontSize)
		tailP = p
	}

	n := copy(tailP, b.buf[len(b.buf)-int(b.backSize)+tailOffset:])
	if tailAmt == n {
		return len(p), nil
	} else {
		size := len(p) - tailAmt + n
		return size, io.EOF
	}
}

func (b *GapBuffer) Read(p []byte) (int, error) {
	n, err := b.ReadAt(p, b.frontSize)
	b.Seek(int64(n), io.SeekCurrent)
	return n, err
}

// searchFor searches forward for the first occurrence of c
// starting at fromOffset (inclusive).
func (b *GapBuffer) searchFor(c byte, fromOffset int) int {
	buf := make([]byte, 1)

	for i := int64(fromOffset); i < b.frontSize+b.backSize; i++ {
		b.ReadAt(buf, int64(i))
		if buf[0] == c {
			return int(i)
		}
	}

	return -1
}

// searchBackFor searches backwards for the first occurrence of c
// starting at fromOffset-1. If no match is found returns -1.
func (b *GapBuffer) searchBackFor(c byte, fromOffset int) int {
	buf := make([]byte, 1)

	for i := fromOffset - 1; i >= 0; i-- {
		b.ReadAt(buf, int64(i))
		if buf[0] == c {
			return i
		}
	}

	return -1
}

// GetLine returns the start and end of the nth line relative to the current position.
// endPos will be the pos of the new line character unless it is the final line
// with no newline character.
// If offset is out of bounds startPos and endPost will be -1
func (b *GapBuffer) GetLine(offset int) (startPos, endPos int) {
	defer func() {
		if startPos > endPos {
			panic(fmt.Sprintf("startPos %d should not be greater than endPos %d", startPos, endPos))
		}
	}()

	pos := b.frontSize

	b.debugPrintf("gb: frontSize: %d\n", pos)

	if offset == 0 {
		start := b.searchBackFor('\n', int(pos)) + 1
		end := b.searchFor('\n', int(pos))

		if end == -1 {
			end = int(b.frontSize + b.backSize - 1)
			if start == end+1 {
				end = start
			}
		}
		return start, end
	} else if offset < 0 {
		newLineCount := 0 - offset

		cur := int(pos)
		end := b.searchFor('\n', cur)
		if end == -1 {
			end = int(b.frontSize + b.backSize - 1)
		}

		for i := 0; i < newLineCount; i++ {
			nl := b.searchBackFor('\n', cur)
			if nl == -1 {
				return -1, -1
			} else {
				end = cur
				cur = nl
			}
		}

		// search back one more to find the start of the current line
		nl := b.searchBackFor('\n', cur)
		return nl + 1, cur
	} else {
		newLineCount := offset + 1
		cur := int(pos)

		start := b.searchBackFor('\n', cur+1) + 1

		for i := 0; i < newLineCount; i++ {
			nl := b.searchFor('\n', cur+1)
			if nl == -1 {
				return -1, -1
			} else {
				start = cur + 1
				cur = nl
			}
		}

		return start, cur
	}
}

// grow the gap size by at least minExpansion
func (b *GapBuffer) grow(minExpansion int) {
	newSize := len(b.buf)
	for newSize < len(b.buf)+minExpansion {
		newSize = newSize * 2
	}
	newBuf := make([]byte, newSize)

	copy(newBuf[0:], b.buf[:b.frontSize])
	copy(newBuf[len(newBuf)-int(b.backSize):], b.buf[len(b.buf)-int(b.backSize):])
	b.buf = newBuf
}

func (b *GapBuffer) gapSize() int {
	return len(b.buf) - int(b.frontSize) - int(b.backSize)
}

func (b *GapBuffer) moveCursor(relative int64) {
	newFront := b.frontSize + relative
	newBack := b.backSize - relative
	if relative < 0 {
		copy(b.buf[len(b.buf)-int(newBack):], b.buf[newFront:b.frontSize])
	} else {
		copy(b.buf[b.frontSize:], b.buf[len(b.buf)-int(b.backSize):len(b.buf)-int(newBack)])
	}

	b.frontSize = newFront
	b.backSize = newBack
}

type debugInfo struct {
	Front   []byte
	Back    []byte
	Cap     int
	GapSize int
}

func (i debugInfo) String() string {
	return fmt.Sprintf("front:%q back:%q cap:%d gapSize:%d", i.Front, i.Back, i.Cap, i.GapSize)
}

func (i debugInfo) Bytes() []byte {
	b := make([]byte, 0, len(i.Front)+len(i.Back))
	b = append(b, i.Front...)
	b = append(b, i.Back...)
	return b
}

func (b *GapBuffer) debugInfo() debugInfo {
	return debugInfo{
		Front:   b.buf[:b.frontSize],
		Back:    b.buf[len(b.buf)-int(b.backSize):],
		Cap:     len(b.buf),
		GapSize: len(b.buf) - int(b.frontSize) - int(b.backSize),
	}
}

// XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
func (b *GapBuffer) DebugInfo() debugInfo {
	return b.debugInfo()
}

func (b *GapBuffer) debugPrintf(s string, args ...any) {
	if b.Debug != nil {
		fmt.Fprintf(b.Debug, s, args...)
	}
}
