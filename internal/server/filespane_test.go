package server

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/gittest"
	"github.com/emmyfong/snow/pkg/api"
)

// openFilesPane opens a files pane on dir and returns its ID with the
// client.
func openFilesPane(t *testing.T, dir string) (*apitest.Client, int) {
	t.Helper()
	s := newTestServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "files", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpOpen, Target: first, Args: map[string]string{api.ArgKind: api.PaneFiles}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneFiles {
			c.WaitSide(p.ID, func(s *api.SidePaneState) bool {
				var st api.FilesState
				return api.DecodeSideState(s, &st) == nil && !st.Loading
			})
			return c, p.ID
		}
	}
	t.Fatal("no files pane")
	return nil, 0
}

func filesState(t *testing.T, s *api.SidePaneState) api.FilesState {
	t.Helper()
	var st api.FilesState
	if err := api.DecodeSideState(s, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestFilesKindOpens: the server lists the files kind, opens it, and sends
// the folder's top level.
func TestFilesKindOpens(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, pane := openFilesPane(t, dir)
	if !slices.ContainsFunc(c.Kinds(), func(k api.PaneKindInfo) bool { return k.Name == api.PaneFiles }) {
		t.Fatalf("kinds %+v do not list files", c.Kinds())
	}
	st := filesState(t, c.Side(pane))
	if got := entryNames(st, ""); !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("root entries %q, want a.txt", got)
	}
	if !reflect.DeepEqual(st.Expanded, []string{""}) {
		t.Fatalf("expanded %q, want only the root", st.Expanded)
	}
}

// TestTreeLazyExpand: a folder's children are read when it opens, and
// dropped again when it closes.
func TestTreeLazyExpand(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("in"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, pane := openFilesPane(t, dir)
	st := filesState(t, c.Side(pane))
	if got := folderPaths(st); !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("folders %q before expanding, want the root only", got)
	}

	c.Send(&api.SidePaneAction{Pane: pane, Action: api.FilesActExpand, Args: map[string]string{api.ArgPath: "sub"}})
	s := c.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.FilesState
		return api.DecodeSideState(s, &st) == nil && hasFolder(st, "sub")
	})
	st = filesState(t, s)
	if got := entryNames(st, "sub"); !reflect.DeepEqual(got, []string{"inner.txt"}) {
		t.Fatalf("sub entries %q, want inner.txt", got)
	}

	c.Send(&api.SidePaneAction{Pane: pane, Action: api.FilesActCollapse, Args: map[string]string{api.ArgPath: "sub"}})
	s = c.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.FilesState
		return api.DecodeSideState(s, &st) == nil && !hasFolder(st, "sub")
	})
	st = filesState(t, s)
	if !reflect.DeepEqual(st.Expanded, []string{""}) {
		t.Fatalf("expanded %q after collapsing, want the root only", st.Expanded)
	}
}

// TestFilesGitStatusMarks: the pane's entries carry git's marks.
func TestFilesGitStatusMarks(t *testing.T) {
	dir := t.TempDir()
	gittest.Repo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, pane := openFilesPane(t, dir)
	s := c.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.FilesState
		return api.DecodeSideState(s, &st) == nil && markOf(st, "a.txt") == "M"
	})
	st := filesState(t, s)
	if got := markOf(st, "a.txt"); got != "M" {
		t.Fatalf("a.txt mark %q, want M", got)
	}
	if got := markOf(st, "new.txt"); got != "?" {
		t.Fatalf("new.txt mark %q, want ?", got)
	}
}

// TestFilesNotARepoNoMarks: outside a repository the tree still opens and
// has no marks.
func TestFilesNotARepoNoMarks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, pane := openFilesPane(t, dir)
	st := filesState(t, c.Side(pane))
	if got := markOf(st, "a.txt"); got != "" {
		t.Fatalf("a.txt mark %q, want none", got)
	}
	if st.Error != "" {
		t.Fatalf("state error %q, want none", st.Error)
	}
}

// TestFilesOpenFileSelects: open-file opens the file's folders and selects
// it.
func TestFilesOpenFileSelects(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, pane := openFilesPane(t, dir)
	c.Send(&api.Command{Op: api.OpOpenFile, Target: pane, Args: map[string]string{api.ArgPath: "a/b/c.txt"}})
	s := c.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.FilesState
		return api.DecodeSideState(s, &st) == nil && st.Selected == "a/b/c.txt"
	})
	st := filesState(t, s)
	for _, want := range []string{"", "a", "a/b"} {
		if !hasFolder(st, want) {
			t.Fatalf("folders %q, want %q open", folderPaths(st), want)
		}
	}
}

// TestFilesRejectsTraversal: an action cannot leave the pane's folder.
func TestFilesRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	c, pane := openFilesPane(t, dir)
	c.Send(&api.SidePaneAction{Pane: pane, Action: api.FilesActExpand, Args: map[string]string{api.ArgPath: "../x"}})
	if e := nextError(t, c); e.Code != api.CodeCommandFailed {
		t.Fatalf("got %+v, want a command failure", e)
	}
}

func entryNames(st api.FilesState, folder string) []string {
	for _, f := range st.Folders {
		if f.Path != folder {
			continue
		}
		var out []string
		for _, e := range f.Entries {
			out = append(out, e.Name)
		}
		return out
	}
	return nil
}

func folderPaths(st api.FilesState) []string {
	var out []string
	for _, f := range st.Folders {
		out = append(out, f.Path)
	}
	return out
}

func hasFolder(st api.FilesState, path string) bool {
	for _, f := range st.Folders {
		if f.Path == path {
			return true
		}
	}
	return false
}

func markOf(st api.FilesState, path string) string {
	for _, f := range st.Folders {
		for _, e := range f.Entries {
			if e.Path == path {
				return e.Mark
			}
		}
	}
	return ""
}
