//go:build !windows

package server

import (
	"os"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestSaveFailureShows: while saving fails, the home page says so; once a
// save works again, the note goes away. A read-only folder makes saves
// fail (Unix only: Windows ignores the folder mode).
func TestSaveFailureShows(t *testing.T) {
	s, _ := persistServer(t)
	dir := s.opts.SessionsDir
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	home := connect(t, s, 80, 24)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	nextList(t, home, func(l *api.SessionList) bool {
		return len(l.Notes) == 1 && strings.Contains(l.Notes[0], "not being saved")
	})
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession("b"); err != nil {
		t.Fatal(err)
	}
	nextList(t, home, func(l *api.SessionList) bool { return len(l.Notes) == 0 })
}
