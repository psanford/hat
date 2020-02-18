package gapbuffer

import (
	"errors"
	"io"
)

type GapBuffer struct {
	buf       []byte
	frontSize int64
	backSize  int64
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
// starting at fromOffset-1.
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
func (b *GapBuffer) GetLine(offset int) (startPos, endPos int) {
	pos := b.frontSize

	if offset == 0 {
		start := b.searchBackFor('\n', int(pos)) + 1
		end := b.searchFor('\n', int(pos))

		if end == -1 {
			end = int(b.frontSize + b.backSize - 1)
		}
		return start, end
	} else if offset < 0 {
		newLineCount := 0 - offset
		newLineCount++

		cur := int(pos)

		end := b.searchFor('\n', cur)
		if end == -1 {
			end = int(b.frontSize + b.backSize - 1)
		}

		for i := 0; i < newLineCount; i++ {
			nl := b.searchBackFor('\n', cur)
			if nl == -1 {
				return 0, end
			} else {
				cur = nl
				end = cur
			}
		}

		return cur, end
	} else {
		newLineCount := offset + 1
		cur := int(pos)

		start := b.searchBackFor('\n', cur)
		if start == -1 {
			start = 0
		}

		for i := 0; i < newLineCount; i++ {
			nl := b.searchFor('\n', cur)
			if nl == -1 {
				return start, int(b.frontSize + b.backSize - 1)
			} else {
				cur = nl
				start = cur
			}
		}

		return start, cur
	}
}

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
