//go:build !windows

package server

import (
	"errors"
	"testing"
)

// TestRoleWSLPath: a role folder with no path inside WSL, as any folder on
// a Unix server, is refused for an agent in a distribution rather than
// handed to claude unreadable.
func TestRoleWSLPath(t *testing.T) {
	s := &Server{opts: Options{RolesDir: t.TempDir()}}
	if _, err := s.roleArgs(Role{Name: "r", Instructions: "Review."}, "Ubuntu"); !errors.Is(err, ErrRoleFolder) {
		t.Fatalf("err %v, want ErrRoleFolder", err)
	}
}
