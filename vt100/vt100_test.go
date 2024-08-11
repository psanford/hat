package vt100

import (
	"bytes"
	"io"
	"testing"
)

func TestReadUntilCursorPosition(t *testing.T) {
	testCases := []struct {
		name        string
		input       []byte
		expectCoord *TermCoord
		expectExtra []byte
		expectError error
	}{
		{
			name:        "Basic cursor position",
			input:       []byte("\x1b[10;20R"),
			expectCoord: &TermCoord{Row: 10, Col: 20},
			expectExtra: []byte{},
			expectError: nil,
		},
		{
			name:        "Cursor position with extra bytes",
			input:       []byte("extra data\x1b[5;15R"),
			expectCoord: &TermCoord{Row: 5, Col: 15},
			expectExtra: []byte("extra data"),
			expectError: nil,
		},
		{
			name:        "No cursor position",
			input:       []byte("just some random data"),
			expectExtra: []byte("just some random data"),
			expectError: io.EOF,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tc.input))
			coord, extraBytes, err := readUntilCursorPosition(reader, 1<<16)

			if coord == nil && tc.expectCoord != nil || coord != nil && tc.expectCoord == nil {
				t.Errorf("Expect coord %+v, got %+v", tc.expectCoord, coord)
			}
			if coord != nil && *coord != *tc.expectCoord {
				t.Errorf("Expect coord %+v, got %+v", tc.expectCoord, coord)
			}

			if !bytes.Equal(extraBytes, tc.expectExtra) {
				t.Errorf("Expect extra bytes %v, got %v", tc.expectExtra, extraBytes)
			}
			if (err == nil && tc.expectError != nil) || (err != nil && tc.expectError == nil) || (err != nil && tc.expectError != nil && err.Error() != tc.expectError.Error()) {
				t.Errorf("Expect error %v, got %v", tc.expectError, err)
			}
		})
	}
}
