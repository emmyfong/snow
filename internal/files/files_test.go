package files

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestTreeListsFolder: a listing holds one folder's children, folders
// first, and leaves out what .gitignore ignores.
func TestTreeListsFolder(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "*.log\nbuild/\n")
	write("b.txt", "b")
	write("a.txt", "a")
	write("skip.log", "no")
	write("dir/inner.txt", "inner")
	write("dir/skip.tmp", "no")
	write("dir/.gitignore", "*.tmp\n")
	write("build/out.txt", "no")
	write(".git/config", "no")

	got, omitted, err := List(context.Background(), root, "")
	if err != nil || omitted != 0 {
		t.Fatalf("List: %v, omitted %d", err, omitted)
	}
	var names []string
	for _, e := range got {
		names = append(names, e.Name)
	}
	want := []string{"dir", ".gitignore", "a.txt", "b.txt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("root entries %q, want %q", names, want)
	}

	got, _, err = List(context.Background(), root, "dir")
	if err != nil {
		t.Fatal(err)
	}
	names = nil
	for _, e := range got {
		names = append(names, e.Name)
	}
	want = []string{".gitignore", "inner.txt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("dir entries %q, want %q; a nested .gitignore must hold", names, want)
	}
}

// TestGitStatusMarks: a repository's marks name the file's change, from
// the pane's folder down.
func TestGitStatusMarks(t *testing.T) {
	root := t.TempDir()
	gittest.Repo(t, root)
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("gone.txt", "gone")
	gittest.Run(t, root, "add", "gone.txt")
	gittest.Run(t, root, "commit", "-q", "-m", "gone")
	write("a.txt", "changed\n")
	write("new.txt", "new\n")
	write("staged.txt", "staged\n")
	gittest.Run(t, root, "add", "staged.txt")
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}

	marks, err := GitMarks(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.txt": "M", "new.txt": "?", "staged.txt": "A", "gone.txt": "D"}
	if !reflect.DeepEqual(marks, want) {
		t.Fatalf("marks %v, want %v", marks, want)
	}
}

// TestGitMarksBelowRepositoryRoot: a pane opened on a subfolder keeps only
// that folder's paths.
func TestGitMarksBelowRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	gittest.Repo(t, root)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "in.txt"), []byte("in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "out.txt"), []byte("out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marks, err := GitMarks(context.Background(), sub, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"in.txt": "?"}
	if !reflect.DeepEqual(marks, want) {
		t.Fatalf("marks %v, want %v", marks, want)
	}
}

// TestNotARepoNoMarks: a folder outside git has no marks and no error.
func TestNotARepoNoMarks(t *testing.T) {
	marks, err := GitMarks(context.Background(), t.TempDir(), "")
	if err != nil || len(marks) != 0 {
		t.Fatalf("marks %v, %v; want none", marks, err)
	}
}

// TestListLeavesOutPastTheCap: a folder with more than MaxEntries names
// how many it left out.
func TestListLeavesOutPastTheCap(t *testing.T) {
	if testing.Short() {
		t.Skip("writes MaxEntries files")
	}
	root := t.TempDir()
	for i := range MaxEntries + 3 {
		name := filepath.Join(root, "f"+strconv.Itoa(i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, omitted, err := List(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxEntries || omitted != 3 {
		t.Fatalf("%d entries, omitted %d; want %d and 3", len(entries), omitted, MaxEntries)
	}
}
