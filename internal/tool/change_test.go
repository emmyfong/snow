package tool

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// TestWriteKeepsLinkAndMode: writing through a link inside the folder
// changes the target and keeps the link, the mode, and no temp file.
func TestWriteKeepsLinkAndMode(t *testing.T) {
	env := tree(t, map[string]string{"real.txt": "old\n"})
	target := filepath.Join(env.Folder.Path, "real.txt")
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(env.Folder.Path, "link.txt")
	if err := os.Symlink("real.txt", link); err != nil {
		t.Skip("cannot make symlinks here")
	}
	if got := run(t, env, "write", `{"path":"link.txt","content":"new\n"}`); got.IsError {
		t.Fatalf("got %+v", got)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	if s := readFile(t, env, "real.txt"); s != "new\n" {
		t.Fatalf("target %q", s)
	}
	if fi, _ := os.Stat(target); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(env.Folder.Path)
	if len(entries) != 2 {
		t.Fatalf("left behind: %v", entries)
	}
}

// TestReadLongLineBounded: a huge line with no newline is cut, and read
// does not hold it all.
func TestReadLongLineBounded(t *testing.T) {
	env := tree(t, map[string]string{"big": strings.Repeat("x", 8<<20)})
	got := run(t, env, "read", `{"path":"big"}`)
	if got.IsError || len(got.Content) > readLineChars+100 || !strings.Contains(got.Content, "…") {
		t.Fatalf("%d bytes, error %v", len(got.Content), got.IsError)
	}
}
