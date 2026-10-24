package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func readFile(t *testing.T, env Env, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(env.Folder.Path, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEditOnce(t *testing.T) {
	env := tree(t, map[string]string{"a.go": "one\ntwo\nthree\n", "crlf.txt": "a\r\nb\r\nc\r\n"})
	got := run(t, env, "edit", `{"path":"a.go","old":"two\n","new":"2\n2b\n"}`)
	if got.IsError || got.Content != "edited a.go: +2 -1 lines\n" {
		t.Fatalf("got %+v", got)
	}
	if s := readFile(t, env, "a.go"); s != "one\n2\n2b\nthree\n" {
		t.Fatalf("file %q", s)
	}
	// A CRLF file takes an LF edit and keeps its line endings.
	if got := run(t, env, "edit", `{"path":"crlf.txt","old":"a\nb","new":"a\nB"}`); got.IsError {
		t.Fatalf("CRLF edit: %+v", got)
	}
	if s := readFile(t, env, "crlf.txt"); s != "a\r\nB\r\nc\r\n" {
		t.Fatalf("CRLF file %q", s)
	}
}

func TestEditNoMatch(t *testing.T) {
	env := tree(t, map[string]string{"a.go": "one\n"})
	for in, want := range map[string]string{
		`{"path":"a.go","old":"zzz","new":"y"}`:   "edit a.go: old text not found",
		`{"path":"a.go","old":"","new":"y"}`:      "edit a.go: old text is empty",
		`{"path":"a.go","old":"one","new":"one"}`: "edit a.go: new text is the same as old text",
		`{"path":"no.go","old":"a","new":"b"}`:    "",
	} {
		got := run(t, env, "edit", in)
		if !got.IsError || (want != "" && got.Content != want) {
			t.Errorf("%s: %+v", in, got)
		}
	}
	if s := readFile(t, env, "a.go"); s != "one\n" {
		t.Fatalf("file changed: %q", s)
	}
}

func TestEditManyMatches(t *testing.T) {
	env := tree(t, map[string]string{"a.go": "x\nx\n"})
	if got := run(t, env, "edit", `{"path":"a.go","old":"x","new":"y"}`); !got.IsError || got.Content != "edit a.go: old text appears more than once" {
		t.Fatalf("got %+v", got)
	}
	if s := readFile(t, env, "a.go"); s != "x\nx\n" {
		t.Fatalf("file changed: %q", s)
	}
}

func TestWriteCreatesAndOverwrites(t *testing.T) {
	env := tree(t, map[string]string{"dir/": ""})
	if got := run(t, env, "write", `{"path":"new/sub/a.txt","content":"one\ntwo"}`); got.IsError || got.Content != "created new/sub/a.txt: 2 lines\n" {
		t.Fatalf("create: %+v", got)
	}
	if got := run(t, env, "write", `{"path":"new/sub/a.txt","content":"three\n"}`); got.IsError || got.Content != "overwrote new/sub/a.txt: 1 lines\n" {
		t.Fatalf("overwrite: %+v", got)
	}
	if s := readFile(t, env, "new/sub/a.txt"); s != "three\n" {
		t.Fatalf("file %q", s)
	}
	if got := run(t, env, "write", `{"path":"dir","content":"x"}`); !got.IsError {
		t.Fatalf("writing over a folder: %+v", got)
	}
}
