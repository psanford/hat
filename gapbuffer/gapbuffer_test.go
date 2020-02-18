package gapbuffer

import (
	"fmt"
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

	readLine := func() string {
		start, end := buf.GetLine(0)

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
		t.Fatalf("got %s expect %s", got, expect)
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

	buf.Seek(0, io.SeekEnd)
	got = readLine()
	expect = "debilitates-stalking"
	if string(got) != expect {
		t.Fatalf("got %s expect %s", got, expect)
	}
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

func (b *GapBuffer) debugInfo() debugInfo {
	return debugInfo{
		Front:   b.buf[:b.frontSize],
		Back:    b.buf[len(b.buf)-int(b.backSize):],
		Cap:     len(b.buf),
		GapSize: len(b.buf) - int(b.frontSize) - int(b.backSize),
	}
}
