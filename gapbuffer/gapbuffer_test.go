package gapbuffer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGapbuffer(t *testing.T) {
	buf := newCheckBuffer(2)

	buf.Insert([]byte("ABCDE"))
	buf.Seek(0, io.SeekCurrent)

	buf.Seek(-3, io.SeekCurrent)

	got := buf.debugInfo()
	expect := debugInfo{
		Front:   []byte("AB"),
		Back:    []byte("CDE"),
		GapSize: 3,
		Cap:     8,
	}

	// 0  1  2  3  4  5  6  7
	// A  B  _  _  _  C  D  E
	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

	buf.Insert([]byte("12345678"))

	got = buf.debugInfo()
	expect = debugInfo{
		Front:   []byte("AB12345678"),
		Back:    []byte("CDE"),
		GapSize: 3,
		Cap:     16,
	}

	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

	buf.Delete(2)

	got = buf.debugInfo()
	expect = debugInfo{
		Front:   []byte("AB123456"),
		Back:    []byte("CDE"),
		GapSize: 5,
		Cap:     16,
	}

	if diff := cmp.Diff(got, expect); diff != "" {
		t.Fatal(diff)
	}

	type testCase struct {
		size   int
		offset int
		expect string
		err    error
	}

	cases := []testCase{
		{
			size:   2,
			offset: 0,
			expect: "AB",
			err:    nil,
		},
		{
			size:   2,
			offset: 7,
			expect: "6C",
			err:    nil,
		},
		{
			size:   2,
			offset: 9,
			expect: "DE",
			err:    nil,
		},
		{
			size:   5,
			offset: 8,
			expect: "CDE",
			err:    io.EOF,
		},
		{
			size:   2,
			offset: 10,
			expect: "E",
			err:    io.EOF,
		},
		{
			size:   15,
			offset: 0,
			expect: "AB123456CDE",
			err:    io.EOF,
		},
	}

	for i, tc := range cases {
		tcInfo := fmt.Sprintf("ReadAt (tc=%d size=%d offset=%d)", i, tc.size, tc.offset)
		b := make([]byte, tc.size)
		n, err := buf.ReadAt(b, int64(tc.offset))
		if n != len(tc.expect) {
			t.Errorf("%s: got n=%d expected %d", tcInfo, n, len(tc.expect))
		}
		if err != tc.err {
			t.Errorf("%s: got err=%s expected %s", tcInfo, err, tc.err)
		}
		sb := string(b[:n])
		if sb != tc.expect {
			t.Errorf("%s: got b=%s expected %s", tcInfo, sb, tc.expect)
		}
	}
}

func TestGetLine(t *testing.T) {
	buf := New(50)

	buf.Insert([]byte("Maurice-sibyl\nEarnhardt-oarsman\ninfectiousness-chairman\ndebilitates-stalking"))
	buf.Seek(0, io.SeekStart)

	readLine := func(offs ...int) string {
		var off int
		if len(offs) > 0 {
			off = offs[0]
		}
		start, end := buf.GetLine(off)
		if start == -1 && end == -1 {
			return "<<OUTOFBOUNDS>>"
		}
		if start == end {
			return ""
		}

		got := make([]byte, end-start+1)
		n, err := buf.ReadAt(got, int64(start))
		if err != nil {
			t.Fatal(err)
		}
		if n != len(got) {
			t.Fatalf("Failed to read full buffer size %d != %d", n, len(got))
		}
		return string(got)
	}

	got := readLine()

	expect := "Maurice-sibyl\n"
	if got != expect {
		t.Fatalf("got %q expect %q", got, expect)
	}
	buf.Seek(int64(len(expect)-1), io.SeekStart)

	got = readLine()
	if string(got) != expect {
		t.Fatalf("got %s expect %s", got, expect)
	}

	buf.Seek(1, io.SeekCurrent)
	got = readLine()
	expect = "Earnhardt-oarsman\n"
	if string(got) != expect {
		t.Fatalf("got %s expect %s", got, expect)
	}

	got = readLine(-1)
	expect = "Maurice-sibyl\n"
	if string(got) != expect {
		t.Fatalf("readLine(-1) got %s expect %s", got, expect)
	}

	buf.Seek(0, io.SeekEnd)
	got = readLine()
	expect = "debilitates-stalking"
	if string(got) != expect {
		t.Fatalf("got %s expect %s", got, expect)
	}

	buf = New(10)
	buf.Insert([]byte("cervical-scarf\nwindscreen-materialistic"))
	buf.Seek(int64(len("cervical-scarf\n")), io.SeekStart)
	lastStart, lastEnd := buf.GetLine(0)

	if lastStart != len("cervical-scarf")+1 {
		t.Fatalf("Expected last start of %d but was %d", len("cervical-scarf")+1, lastStart)
	}
	if lastEnd != int(buf.frontSize+buf.backSize-1) {
		t.Fatalf("Expected last end of %d but was %d", buf.frontSize+buf.backSize-1, lastEnd)
	}

	buf = New(2)
	buf.Insert([]byte("expository\ngapes\n"))
	start, end := buf.GetLine(0) // at the end of the file, empy line
	if start != end {
		gotB := make([]byte, end-start+1)
		n, err := buf.ReadAt(gotB, int64(start))
		if err != io.EOF || n != 0 {
			t.Fatalf("expect read 0 at eof but got %d %s", n, err)
		}
		gotB = gotB[:n]
		if string(gotB) != "" {
			t.Fatalf("Expected empty line but got: %+v", gotB)
		}
	}

	// e  x  p  o  s  i  t  o  r  y  \n  g  a  p  e  s  \n
	// 0  1  2  3  4  5  6  7  8  9  10 11 12 13 14 15  16
	start, end = buf.GetLine(-1) // read `grapes\n`
	gotB := make([]byte, end-start+1)
	_, err := buf.ReadAt(gotB, int64(start))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotB) != "gapes\n" {
		t.Fatalf("Expected `gapes\n` line but got: %v <%s>", gotB, gotB)
	}
}

func TestSeek(t *testing.T) {
	buf := newCheckBuffer(2)

	for i := 0; i < 10; i++ {
		cur, _ := buf.Seek(0, io.SeekCurrent)
		if int(cur) != i {
			t.Fatalf("bad cur got=%d expect=%d", cur, i)
		}
		n, _ := buf.Seek(0, io.SeekStart)
		if n != 0 {
			t.Fatalf("bad seek to start got=%d", n)
		}
		n, _ = buf.Seek(cur, io.SeekStart)
		if int(n) != i {
			t.Fatalf("bad seek back to end n=%d expect=%d", n, i)
		}

		s := strconv.Itoa(i)
		buf.Insert([]byte(s))
	}
}

type CheckBuffer struct {
	buf *GapBuffer
	f   *os.File
}

func newCheckBuffer(size int) *CheckBuffer {
	f, err := ioutil.TempFile("", "gapbuffer_checkbuffer")
	if err != nil {
		panic(err)
	}
	return &CheckBuffer{
		buf: New(size),
		f:   f,
	}
}

func (c *CheckBuffer) Seek(offset int64, whence int) (int64, error) {
	bufOff, bufErr := c.buf.Seek(offset, whence)
	fOff, fErr := c.f.Seek(offset, whence)

	if bufOff != fOff || bufErr != fErr {
		panic(fmt.Sprintf("CheckBuffer seek mismatch: boff=%d foff=%d beff=%s ferr=%s", bufOff, fOff, bufErr, fErr))
	}
	return bufOff, bufErr
}

func (c *CheckBuffer) Insert(p []byte) (int, error) {
	n, err := c.buf.Insert(p)

	if c.buf.backSize == 0 {
		c.f.Write(p)
	} else {
		di := c.buf.debugInfo()
		b := di.Bytes()
		ioutil.WriteFile(c.f.Name(), b, 0666)
	}

	return n, err
}

func (c *CheckBuffer) Delete(n int) {
	c.buf.Delete(n)

	di := c.buf.debugInfo()
	b := di.Bytes()
	ioutil.WriteFile(c.f.Name(), b, 0666)
}

func (c *CheckBuffer) ReadAt(p []byte, off int64) (int, error) {
	pp := make([]byte, len(p))
	n, err := c.buf.ReadAt(p, off)

	fn, ferr := c.f.ReadAt(pp, off)

	if n != fn || err != ferr {
		panic(fmt.Sprintf("CheckBuffer ReadAt mismatch: n=%d fn=%d err=%s ferr=%s", n, fn, err, ferr))
	}
	return n, err
}

func (c *CheckBuffer) Read(p []byte) (int, error) {
	pp := make([]byte, len(p))
	n, err := c.buf.Read(p)

	fn, ferr := c.f.Read(pp)

	if n != fn || err != ferr {
		panic(fmt.Sprintf("CheckBuffer Read mismatch: n=%d fn=%d err=%s ferr=%s", n, fn, err, ferr))
	}
	return n, err
}

func (c *CheckBuffer) debugInfo() debugInfo {
	return c.buf.debugInfo()
}
