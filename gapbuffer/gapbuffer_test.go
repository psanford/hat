package gapbuffer

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGapbuffer(t *testing.T) {
	buf := New(2)

	buf.Insert([]byte("ABCDE"))
	buf.Seek(-3, io.SeekCurrent)

	got := buf.debugInfo()
	expect := debugInfo{
		Front:   []byte("AB"),
		Back:    []byte("CDE"),
		GapSize: 5,
		Cap:     10,
	}

	// 0  1  2  3  4  5  6  7  8  9
	// A  B  _  _  _  _  _  C  D  E
	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

	buf.Insert([]byte("12345678"))

	got = buf.debugInfo()
	expect = debugInfo{
		Front:   []byte("AB12345678"),
		Back:    []byte("CDE"),
		GapSize: 7,
		Cap:     20,
	}

	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

	buf.Delete(2)

	got = buf.debugInfo()
	expect = debugInfo{
		Front:   []byte("AB123456"),
		Back:    []byte("CDE"),
		GapSize: 9,
		Cap:     20,
	}

	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

}

type debugInfo struct {
	Front   []byte
	Back    []byte
	Cap     int
	GapSize int
}

func (b *GapBuffer) debugInfo() debugInfo {
	return debugInfo{
		Front:   b.buf[:b.frontSize],
		Back:    b.buf[len(b.buf)-int(b.backSize):],
		Cap:     len(b.buf),
		GapSize: len(b.buf) - int(b.frontSize) - int(b.backSize),
	}
}
