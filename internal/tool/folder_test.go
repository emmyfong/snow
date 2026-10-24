package tool

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveLocal(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "x", "y.go")
	tests := []struct{ in, want string }{
		{"a.go", filepath.Join(root, "a.go")},
		{"sub/../b.go", filepath.Join(root, "b.go")},
		{abs, abs},
	}
	f := Folder{Path: root}
	for _, tt := range tests {
		got, err := f.Resolve(tt.in)
		if err != nil || got.Logical != tt.want || got.OS != tt.want {
			t.Errorf("Resolve(%q) = %+v, %v; want %s", tt.in, got, err, tt.want)
		}
	}
	if _, err := f.Resolve(""); err == nil {
		t.Error("an empty path resolved")
	}
}

func TestResolveWSL(t *testing.T) {
	f := Folder{Path: "/home/u/proj", Distro: "Ubuntu"}
	tests := []struct{ in, logical string }{
		{"main.go", "/home/u/proj/main.go"},
		{`cmd\snow\main.go`, "/home/u/proj/cmd/snow/main.go"},
		{"/etc/hosts", "/etc/hosts"},
	}
	for _, tt := range tests {
		got, err := f.Resolve(tt.in)
		if err != nil || got.Logical != tt.logical {
			t.Errorf("Resolve(%q) = %+v, %v; want %s", tt.in, got, err, tt.logical)
			continue
		}
		want := tt.logical
		if runtime.GOOS == "windows" {
			want = WSLToUNC("Ubuntu", tt.logical)
		}
		if got.OS != want {
			t.Errorf("Resolve(%q).OS = %s, want %s", tt.in, got.OS, want)
		}
	}
}

func TestWSLPaths(t *testing.T) {
	if got := WSLToUNC("Ubuntu", "/home/u"); got != `\\wsl.localhost\Ubuntu\home\u` {
		t.Errorf("WSLToUNC = %s", got)
	}
	tests := []struct {
		in, distro, linux string
		ok                bool
	}{
		{`\\wsl.localhost\Ubuntu\home\u`, "Ubuntu", "/home/u", true},
		{`\\WSL$\Debian\`, "Debian", "/", true},
		{`\\wsl.localhost\`, "", "", false},
		{`C:\Users\u`, "", "", false},
	}
	for _, tt := range tests {
		d, l, ok := UNCToWSL(tt.in)
		if d != tt.distro || l != tt.linux || ok != tt.ok {
			t.Errorf("UNCToWSL(%q) = %q %q %v", tt.in, d, l, ok)
		}
	}
}
