package tool

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBoundaryJunction: a junction, which any user can make, that leads
// outside the folder is outside, also when its target is gone.
func TestBoundaryJunction(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "gone"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"j": outside, "dead": filepath.Join(outside, "gone")} {
		if b, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, name), target).CombinedOutput(); err != nil {
			t.Fatalf("mklink: %v %s", err, b)
		}
	}
	if err := os.Remove(filepath.Join(outside, "gone")); err != nil {
		t.Fatal(err)
	}
	f := Folder{Path: root}
	for _, p := range []string{`j\s.txt`, `j`, `dead\x.txt`} {
		r, err := f.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if f.Contains(r) {
			t.Errorf("%s counts as inside", p)
		}
	}
	if r, _ := f.Resolve("in.txt"); !f.Contains(r) {
		t.Error("a plain file counts as outside")
	}
}
