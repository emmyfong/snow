package server

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// TestFileListHonorsGitignore: a chat's file list leaves out what git
// ignores and holds the normal files.
func TestFileListHonorsGitignore(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "build/\n")
	write("main.go", "package main\n")
	write("build/out.txt", "out\n")
	_, _, c, id := chatInFolder(t, dir)
	c.Send(&api.FileListRequest{Pane: id})
	list, e := apitest.Reply[*api.FileList](c)
	if e != nil {
		t.Fatalf("file list: %+v", e)
	}
	if list.Pane != id || list.Folder != dir || list.Err != "" || list.Cut {
		t.Fatalf("file list %+v", list)
	}
	if slices.Contains(list.Files, "build/out.txt") {
		t.Fatalf("files %q hold an ignored file", list.Files)
	}
	if !slices.Contains(list.Files, "main.go") {
		t.Fatalf("files %q lack main.go", list.Files)
	}
}
