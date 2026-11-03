//go:build linux

package tool

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// TestWSLResolveScript: the resolver script follows links, keeps a missing
// tail, stops a link loop, and resolves several paths in one run. It runs on
// Linux, where readlink -f is the same as in a WSL distro.
func TestWSLResolveScript(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, dir := range []string{"sub", "real"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"link":     filepath.Join("sub", "a"),
		"rel":      filepath.Join("sub", "a"),
		"dirlink":  "sub",
		"out":      filepath.Join(outside, "secret"),
		"dangling": "nowhere",
		"loop":     "loop",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("cannot make symlinks here: %v", err)
		}
	}
	deep := filepath.Join(root, "deep")
	for i := 0; i < 50; i++ {
		deep = filepath.Join(deep, fmt.Sprintf("d%d", i))
	}
	tests := []struct {
		name   string
		in     []string
		want   []string
		isErr  bool
		stderr string
	}{
		{"file", []string{filepath.Join(root, "sub", "a")}, []string{filepath.Join(root, "sub", "a")}, false, ""},
		{"link", []string{filepath.Join(root, "link")}, []string{filepath.Join(root, "sub", "a")}, false, ""},
		{"relative link", []string{filepath.Join(root, "rel")}, []string{filepath.Join(root, "sub", "a")}, false, ""},
		{"link to a folder", []string{filepath.Join(root, "dirlink", "a")}, []string{filepath.Join(root, "sub", "a")}, false, ""},
		{"link outside", []string{filepath.Join(root, "out")}, []string{filepath.Join(outside, "secret")}, false, ""},
		{"dangling link", []string{filepath.Join(root, "dangling")}, []string{filepath.Join(root, "nowhere")}, false, ""},
		{"missing tail", []string{filepath.Join(root, "missing", "dir", "file")}, []string{filepath.Join(root, "missing", "dir", "file")}, false, ""},
		{"missing under a link", []string{filepath.Join(root, "dirlink", "missing")}, []string{filepath.Join(root, "sub", "missing")}, false, ""},
		{"deep missing tail", []string{deep}, []string{deep}, false, ""},
		{"root", []string{"/"}, []string{"/"}, false, ""},
		{"two paths", []string{filepath.Join(root, "link"), filepath.Join(root, "out")}, []string{filepath.Join(root, "sub", "a"), filepath.Join(outside, "secret")}, false, ""},
		{"loop", []string{filepath.Join(root, "loop")}, nil, true, "too many links"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-c", wslResolveScript, "sh"}, tt.in...)
			out, err := exec.Command("sh", args...).Output()
			if (err != nil) != tt.isErr {
				t.Fatalf("err %v, want error %v; output %q", err, tt.isErr, out)
			}
			if tt.isErr {
				var ee *exec.ExitError
				if !errors.As(err, &ee) {
					t.Fatalf("err %v, want an exit error", err)
				}
				if !strings.Contains(string(ee.Stderr), tt.stderr) {
					t.Fatalf("stderr %q, want it to contain %q", ee.Stderr, tt.stderr)
				}
				return
			}
			fields := strings.Split(string(out), "\x00")
			if len(fields) != len(tt.in)+1 || fields[len(fields)-1] != "" {
				t.Fatalf("output %q for %d paths", out, len(tt.in))
			}
			for i, field := range fields[:len(tt.in)] {
				if got := path.Clean(field); got != tt.want[i] {
					t.Errorf("path %d: got %q, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}
