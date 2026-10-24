package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestBoundary(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	links := true
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		links = false // Windows without developer mode cannot make links
	}
	f := Folder{Path: root}
	tests := []struct {
		name, path string
		in         bool
	}{
		{"relative", "sub/a.go", true},
		{"the folder", ".", true},
		{"new file", "sub/new/b.go", true},
		{"dot dot", "../x", false},
		{"dot dot back in", "sub/../a.go", true},
		{"absolute inside", filepath.Join(root, "a.go"), true},
		{"absolute outside", filepath.Join(outside, "a.go"), false},
		{"sibling with the same prefix", root + "-other/a.go", false},
	}
	if links {
		tests = append(tests, struct {
			name, path string
			in         bool
		}{"symlink out", "out/secret", false})
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name, path string
			in         bool
		}{"other case", strings.ToUpper(root) + `\a.go`, true})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := f.Resolve(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got := f.Contains(p); got != tt.in {
				t.Fatalf("Contains(%s) = %v, want %v", p.OS, got, tt.in)
			}
		})
	}
}

func TestBoundaryWSL(t *testing.T) {
	f := Folder{Path: "/home/u/proj", Distro: "Ubuntu"}
	for p, want := range map[string]bool{
		"main.go":            true,
		"/home/u/proj/x":     true,
		"../other":           false,
		"/home/u/project2/x": false,
		`..\..\etc`:          false,
		"/etc/hosts":         false,
	} {
		r, err := f.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.Contains(r); got != want {
			t.Errorf("Contains(%s) = %v, want %v", r.Logical, got, want)
		}
	}
}

func TestOutside(t *testing.T) {
	root := t.TempDir()
	f := Folder{Path: root}
	r := Builtin()
	call := func(name, in string) []string {
		tl, _ := r.Get(name)
		return f.Outside(tl, json.RawMessage(in))
	}
	if got := call("read", `{"path":"../x"}`); !slices.Equal(got, []string{"../x"}) {
		t.Errorf("read outside: %v", got)
	}
	if got := call("list", `{}`); got != nil {
		t.Errorf("list with no path: %v", got)
	}
	if got := call("run", `{"command":"cat /etc/hosts"}`); got != nil {
		t.Errorf("run names no paths: %v", got)
	}
}

func TestPreview(t *testing.T) {
	env := tree(t, map[string]string{"a.go": "one\ntwo\n"})
	edit, _ := Builtin().Get("edit")
	c, err := edit.(Previewer).Preview(env, json.RawMessage(`{"path":"a.go","old":"two","new":"2"}`))
	if err != nil || c != (Change{Path: "a.go", Old: "one\ntwo\n", New: "one\n2\n"}) {
		t.Fatalf("edit preview %+v, %v", c, err)
	}
	if s := readFile(t, env, "a.go"); s != "one\ntwo\n" {
		t.Fatalf("preview changed the file: %q", s)
	}
	write, _ := Builtin().Get("write")
	c, err = write.(Previewer).Preview(env, json.RawMessage(`{"path":"new.go","content":"x"}`))
	if err != nil || c != (Change{Path: "new.go", New: "x"}) {
		t.Fatalf("write preview %+v, %v", c, err)
	}
}
