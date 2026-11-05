package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoleWSLPath: an agent in a WSL distribution gets the instructions
// file at its /mnt path, which reads the same file, a blank in the folder
// name included.
func TestRoleWSLPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Emmy Fong", "roles")
	s := &Server{opts: Options{RolesDir: dir}}
	args, err := s.roleArgs(Role{Name: "Code Reviewer", Instructions: "Review."}, "Ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	got := args[len(args)-1]
	drive := strings.ToLower(dir[:1])
	if !strings.HasPrefix(got, "/mnt/"+drive+"/") || !strings.Contains(got, "/Emmy Fong/roles/") {
		t.Fatalf("path %q, want the folder under /mnt/%s", got, drive)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(files) != 1 {
		t.Fatalf("files %v", files)
	}
	if b, err := os.ReadFile(files[0]); err != nil || string(b) != "Review." {
		t.Fatalf("file holds %q, %v", b, err)
	}
}
