package convstore

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

// TestCreate: a new conversation file starts with its meta record, with
// the format version stamped, before Create returns.
func TestCreate(t *testing.T) {
	s := openStore(t)
	meta := *testMeta()
	meta.Version = 0
	w, err := s.Create(meta)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	rs := records(t, w.Path())
	if len(rs) != 1 {
		t.Fatalf("file holds %d records before any Append", len(rs))
	}
	if m, ok := rs[0].(*Meta); !ok || m.ID != testID || m.Version != Version {
		t.Fatalf("first record %#v", rs[0])
	}
}

// TestCreateNewID: a meta record with no ID gets a new one.
func TestCreateNewID(t *testing.T) {
	s := openStore(t)
	meta := *testMeta()
	meta.ID = ""
	w, err := s.Create(meta)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := CheckID(w.ID()); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Load(w.ID()); err != nil || c.Meta.ID != w.ID() {
		t.Fatalf("load %s: %v", w.ID(), err)
	}
}

// TestCreateExisting: Create never adds to a file that exists.
func TestCreateExisting(t *testing.T) {
	s := openStore(t)
	w := openWriter(t, s)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(*testMeta()); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("got %v, want fs.ErrExist", err)
	}
}

// TestWriterNeedsFile: Writer resumes a conversation; it never makes a
// file without a meta record.
func TestWriterNeedsFile(t *testing.T) {
	s := openStore(t)
	var we *WriteError
	if _, err := s.Writer(testID); !errors.As(err, &we) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got %v, want a WriteError for a missing file", err)
	}
	path, _ := s.Path(testID)
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Writer created %s: %v", path, err)
	}
}

// TestAppendRefusesMeta: only Create writes a meta record.
func TestAppendRefusesMeta(t *testing.T) {
	w := openWriter(t, openStore(t))
	var we *WriteError
	if err := w.Append(testMeta()); !errors.As(err, &we) {
		t.Fatalf("got %v, want a WriteError", err)
	}
	if n := len(records(t, w.Path())); n != 1 {
		t.Fatalf("file holds %d records", n)
	}
}
