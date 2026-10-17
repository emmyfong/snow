package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestList finds every saved session and reports, without hiding, a file
// that cannot be read back. Temp files from an interrupted write are not
// sessions.
func TestList(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"api", "web app"} {
		if err := s.Save(Snapshot{Name: name, Windows: []Window{{Name: "sh", Panes: []Pane{{ID: 1}}}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api.json.tmp"), []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	infos, errs := s.List()
	if len(infos) != 2 || infos[0].Name != "api" || infos[1].Name != "web app" || infos[0].Windows != 1 || infos[0].Saved.IsZero() {
		t.Fatalf("List() = %+v, want api and web app with their windows and save times", infos)
	}
	var de *DecodeError
	if len(errs) != 1 || !errors.As(errs[0], &de) || filepath.Base(de.Path) != "bad.json" {
		t.Fatalf("List() errors %v, want one DecodeError for bad.json", errs)
	}
}

// TestBrokenSnapshotMoved: a file that cannot be read back moves to
// broken/, stamped with the time, so it is kept for the user and no longer
// in the way.
func TestBrokenSnapshotMoved(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 8, 9, 14, 0, 0, time.UTC)
	moved, err := s.MoveBroken(bad, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "broken", "bad-20261008T091400.json"); moved != want {
		t.Fatalf("moved to %s, want %s", moved, want)
	}
	if _, err := os.Stat(bad); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the broken file is still in the sessions folder")
	}
	if b, err := os.ReadFile(moved); err != nil || string(b) != "{not json" {
		t.Fatalf("moved file: %q, %v; want the content kept", b, err)
	}
	if infos, errs := s.List(); len(infos) != 0 || len(errs) != 0 {
		t.Fatalf("List after the move: %v, %v; want nothing", infos, errs)
	}
}
