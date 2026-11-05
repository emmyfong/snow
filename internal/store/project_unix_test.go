//go:build !windows

package store

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRememberFileMode(t *testing.T) {
	p, err := OpenProjects(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveLayout("k", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p.path("k"))
	if err != nil {
		t.Fatal(err)
	}
	if m := fi.Mode().Perm(); m != 0o600 {
		t.Fatalf("mode %o, want 600", m)
	}
}
