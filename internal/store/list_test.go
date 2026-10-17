package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
