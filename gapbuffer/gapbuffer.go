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
		copy(b.buf[int(newFront):], b.buf[len(b.buf)-int(b.backSize):len(b.buf)-int(newBack)])
	}

	b.frontSize = newFront
	b.backSize = newBack
}
