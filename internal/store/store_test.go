package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSaveLoad(t *testing.T) {
	st := openStore(t)
	in := sample("API dev")
	if err := st.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := st.Load("API dev")
	if err != nil {
		t.Fatal(err)
	}
	if out.Saved.IsZero() {
		t.Fatal("Save did not stamp the saved time")
	}
	if out.Name != in.Name || len(out.Windows) != len(in.Windows) {
		t.Fatalf("Load returned %+v, want %+v", out, in)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := openStore(t).Load("api")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load of a missing session: %v, want fs.ErrNotExist", err)
	}
}

func TestLoadBroken(t *testing.T) {
	st := openStore(t)
	path, err := st.path("api")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = st.Load("api")
	var de *DecodeError
	if !errors.As(err, &de) || de.Path != path {
		t.Fatalf("Load of a broken file: %v, want a *DecodeError for %s", err, path)
	}
}

func TestSaveRejectsBadName(t *testing.T) {
	if err := openStore(t).Save(Snapshot{}); !errors.Is(err, ErrBadName) {
		t.Fatalf("Save with no name: %v, want ErrBadName", err)
	}
}

// TestKillDeletesSnapshot covers the store side of killing a session. The
// server side, kill calls Saver.Delete, waits for #52.
func TestKillDeletesSnapshot(t *testing.T) {
	st := openStore(t)
	if err := st.Save(sample("api")); err != nil {
		t.Fatal(err)
	}
	path, err := st.path("api")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tempPath(path), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("api"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, tempPath(path)} {
		if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s still exists after Delete: %v", filepath.Base(p), err)
		}
	}
	if _, err := st.Load("api"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load after Delete: %v, want fs.ErrNotExist", err)
	}
	if err := st.Delete("api"); err != nil {
		t.Fatalf("second Delete: %v, want nil", err)
	}
}
