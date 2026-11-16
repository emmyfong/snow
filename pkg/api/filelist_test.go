package api

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

// TestFileListValidate: a request names a chat pane.
func TestFileListValidate(t *testing.T) {
	tests := []struct {
		name string
		pane int
		ok   bool
	}{
		{"pane", 1, true},
		{"no pane", 0, false},
		{"negative pane", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&FileListRequest{Pane: tt.pane}).Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidFileList) {
				t.Fatalf("error %v does not wrap ErrInvalidFileList", err)
			}
		})
	}
}

// TestFileListRoundTrip: the request and answer survive a frame, and an
// empty answer encodes Files as [], never null.
func TestFileListRoundTrip(t *testing.T) {
	for _, m := range []Message{
		&FileListRequest{Pane: 2},
		&FileList{Pane: 2, Folder: "/work/project", Files: []string{"a.txt", "sub/b.txt"}, Cut: true},
		&FileList{Pane: 2, Files: []string{}, Err: "no such folder"},
	} {
		var buf bytes.Buffer
		if err := WriteFrame(&buf, m); err != nil {
			t.Fatal(err)
		}
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("round trip = %#v, want %#v", got, m)
		}
	}
	if b, want := mustJSON(t, &FileList{Pane: 1}), `{"pane":1,"files":[]}`; b != want {
		t.Fatalf("empty file list is %s, want %s", b, want)
	}
}

// TestFileListMessageNumbers pins the numbers: a released type is never
// renumbered.
func TestFileListMessageNumbers(t *testing.T) {
	if TypeFileListRequest != 51 || TypeFileList != 52 {
		t.Fatalf("numbers %d, %d", TypeFileListRequest, TypeFileList)
	}
}
