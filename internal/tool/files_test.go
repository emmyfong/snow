package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes files under a new folder. A name ending in / is a folder.
func tree(t *testing.T, files map[string]string) Env {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Env{Folder: Folder{Path: root}}
}

// run calls the built-in tool name with the input in.
func run(t *testing.T, env Env, name, in string) Result {
	t.Helper()
	return Builtin().Call(context.Background(), env, name, json.RawMessage(in), nil)
}

func TestBuiltinTools(t *testing.T) {
	changes := map[string]bool{"ask": false, "read": false, "list": false, "search": false, "edit": true, "write": true, "run": true}
	for name, want := range changes {
		tl, ok := Builtin().Get(name)
		if !ok {
			t.Fatalf("%s is not built in", name)
		}
		d := tl.Def()
		if d.Changes != want || d.Description == "" || !json.Valid(d.Schema) {
			t.Errorf("%s: %+v", name, d)
		}
	}
}

func TestReadRange(t *testing.T) {
	var long strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&long, "line %d\n", i)
	}
	env := tree(t, map[string]string{
		"a.txt":    "one\r\ntwo\nthree",
		"long.txt": long.String(),
		"bin":      "x\x00y",
		"empty":    "",
		"dir/":     "",
	})
	tests := []struct {
		name, in, want string
		isErr          bool
	}{
		{"numbers lines", `{"path":"a.txt"}`, "     1\tone\n     2\ttwo\n     3\tthree\n", false},
		{"offset and limit", `{"path":"a.txt","offset":2,"limit":1}`, "     2\ttwo\n[cut: showing lines 2-2 of 3; read more with offset 3]\n", false},
		{"empty", `{"path":"empty"}`, "(empty file)\n", false},
		{"past the end", `{"path":"a.txt","offset":9}`, "a.txt has 3 lines; offset 9 is past the end", true},
		{"binary", `{"path":"bin"}`, "bin is a binary file", true},
		{"folder", `{"path":"dir"}`, "dir is a folder; use list", true},
		{"unknown field", `{"path":"a.txt","lines":3}`, `bad input: json: unknown field "lines"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, env, "read", tt.in)
			if got.IsError != tt.isErr || got.Content != tt.want {
				t.Fatalf("got %+v\nwant %q", got, tt.want)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		if got := run(t, env, "read", `{"path":"nope"}`); !got.IsError || !strings.HasPrefix(got.Content, "read nope: ") {
			t.Fatalf("got %+v", got)
		}
	})
}

// TestReadCap: a file longer than the default limit ends with a notice
// that says how to read on.
func TestReadCap(t *testing.T) {
	var long strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&long, "line %d\n", i)
	}
	env := tree(t, map[string]string{"long.txt": long.String()})
	got := run(t, env, "read", `{"path":"long.txt"}`)
	if !strings.HasSuffix(got.Content, "  2000\tline 2000\n[cut: showing lines 1-2000 of 3000; read more with offset 2001]\n") {
		t.Fatalf("ends with %q", got.Content[len(got.Content)-100:])
	}
}

func TestListGitignore(t *testing.T) {
	env := tree(t, map[string]string{
		".gitignore":     "*.log\nbuild/\n",
		"main.go":        "",
		"app.log":        "",
		"build/out":      "",
		"sub/.gitignore": "secret.txt\n",
		"sub/secret.txt": "",
		"sub/keep.txt":   "",
		".git/HEAD":      "",
		"empty/":         "",
	})
	got := run(t, env, "list", `{}`)
	want := ".gitignore\nempty/\nmain.go\nsub/\nsub/.gitignore\nsub/keep.txt\n"
	if got.IsError || got.Content != want {
		t.Fatalf("got %q\nwant %q", got.Content, want)
	}
	if got := run(t, env, "list", `{"path":"empty"}`); got.Content != "(empty folder)\n" {
		t.Fatalf("empty folder: %q", got.Content)
	}
	if got := run(t, env, "list", `{"path":"main.go"}`); !got.IsError {
		t.Fatalf("listing a file: %+v", got)
	}
}

// TestListCap: a folder with more entries than the cap says how many it
// left out.
func TestListCap(t *testing.T) {
	files := map[string]string{}
	for i := range listEntries + 5 {
		files[fmt.Sprintf("f%04d", i)] = ""
	}
	got := run(t, tree(t, files), "list", `{}`)
	if !strings.HasSuffix(got.Content, "[cut: 5 more entries not shown; list a subfolder]\n") {
		t.Fatalf("ends with %q", got.Content[len(got.Content)-80:])
	}
}

func TestSearch(t *testing.T) {
	env := tree(t, map[string]string{
		".gitignore":  "vendor/\n",
		"a.go":        "package a\nfunc Hello() {}\n",
		"b.txt":       "hello world\n",
		"vendor/v.go": "func Hello() {}\n",
		"bin":         "Hello\x00",
	})
	tests := []struct {
		name, in, want string
		isErr          bool
	}{
		{"pattern", `{"pattern":"Hello"}`, "a.go:2: func Hello() {}\n", false},
		{"ignore case", `{"pattern":"hello","ignore_case":true}`, "a.go:2: func Hello() {}\nb.txt:1: hello world\n", false},
		{"glob", `{"pattern":"hello","ignore_case":true,"glob":"*.txt"}`, "b.txt:1: hello world\n", false},
		{"one file", `{"pattern":"world","path":"b.txt"}`, "b.txt:1: hello world\n", false},
		{"no match", `{"pattern":"zzz"}`, "no matches\n", false},
		{"bad pattern", `{"pattern":"("}`, "bad pattern: error parsing regexp: missing closing ): `(`", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, env, "search", tt.in)
			if got.IsError != tt.isErr || got.Content != tt.want {
				t.Fatalf("got %+v\nwant %q", got, tt.want)
			}
		})
	}
}

// TestSearchCap: a broad search stops at the cap and says so.
func TestSearchCap(t *testing.T) {
	got := run(t, tree(t, map[string]string{"a": strings.Repeat("x\n", searchMatches+10)}), "search", `{"pattern":"x"}`)
	if n := strings.Count(got.Content, "\n"); n != searchMatches+1 || !strings.HasSuffix(got.Content, "narrow the pattern, path, or glob]\n") {
		t.Fatalf("%d lines, ends with %q", n, got.Content[len(got.Content)-60:])
	}
}

// TestToolsStopOnCancel: a canceled context stops a walk.
func TestToolsStopOnCancel(t *testing.T) {
	env := tree(t, map[string]string{"a": "x\n"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, name := range []string{"list", "search"} {
		in := `{}`
		if name == "search" {
			in = `{"pattern":"x"}`
		}
		if got := Builtin().Call(ctx, env, name, json.RawMessage(in), nil); !got.IsError || !strings.Contains(got.Content, "context canceled") {
			t.Errorf("%s: %+v", name, got)
		}
	}
}
