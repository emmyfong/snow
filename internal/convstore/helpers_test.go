package convstore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// at is a fixed time for records in tests.
var at = time.Date(2026, 10, 23, 19, 15, 0, 0, time.UTC)

// testID is a valid conversation ID for tests.
const testID = "20261023-191500-a1b2c3d4"

// testMeta returns the meta record of testID.
func testMeta() *Meta {
	return &Meta{Version: Version, ID: testID, Folder: "/home/u/p", Created: at, Model: "anthropic/claude-opus-5-5"}
}

// fixture copies testdata/name to the file of testID in s and returns its
// path.
func fixture(t *testing.T, s *Store, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.Path(testID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// openStore returns a store in a new temporary folder.
func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "conversations"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// openWriter creates testID with testMeta and returns its Writer, which the
// test closes.
func openWriter(t *testing.T, s *Store) *Writer {
	t.Helper()
	w, err := s.Create(*testMeta())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// fileLines returns the lines of the file at path without their newlines.
// A last line with no newline is returned too, and torn reports it.
func fileLines(t *testing.T, path string) (lines [][]byte, torn bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	torn = len(data) > 0 && data[len(data)-1] != '\n'
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(l) > 0 {
			lines = append(lines, l)
		}
	}
	return lines, torn
}

// records decodes every line of the file at path and fails on a bad one.
func records(t *testing.T, path string) []Record {
	t.Helper()
	lines, torn := fileLines(t, path)
	if torn {
		t.Fatalf("%s ends in the middle of a line", filepath.Base(path))
	}
	var rs []Record
	for i, l := range lines {
		r, err := Decode(l)
		if err != nil {
			t.Fatalf("line %d %q: %v", i+1, l, err)
		}
		rs = append(rs, r)
	}
	return rs
}

// partials returns the texts of the Partial records in the file at path.
func partials(t *testing.T, path string) []string {
	t.Helper()
	var texts []string
	for _, r := range records(t, path) {
		if p, ok := r.(*Partial); ok {
			texts = append(texts, p.Text)
		}
	}
	return texts
}
