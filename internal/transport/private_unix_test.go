//go:build !windows

package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestSocketMode(t *testing.T) {
	path := socketPath(t)
	listen(t, path)
	if got := mode(t, path); got != 0o600 {
		t.Fatalf("socket mode %o, want 600", got)
	}
}

func TestSocketFolderPrivate(t *testing.T) {
	path := socketPath(t)
	listen(t, path)
	if got := mode(t, filepath.Dir(path)); got != 0o700 {
		t.Fatalf("folder mode %o, want 700", got)
	}
}

func TestPrivateFolderTightensLooseMode(t *testing.T) {
	path := socketPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	listen(t, path)
	if got := mode(t, filepath.Dir(path)); got != 0o700 {
		t.Fatalf("folder mode %o after Listen, want 700", got)
	}
}

func TestListenRejectsSymlinkedFolder(t *testing.T) {
	path := socketPath(t)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Dir(path)); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen accepted a symlinked socket folder")
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	tests := []struct {
		name, xdg, want string
	}{
		{"runtime dir", "/run/user/1000", "/run/user/1000/snow/default.sock"},
		{"home fallback", "", filepath.Join(home, ".snow", "run", "default.sock")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_RUNTIME_DIR", tt.xdg)
			t.Setenv("HOME", home)
			got, err := DefaultPath()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("DefaultPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
