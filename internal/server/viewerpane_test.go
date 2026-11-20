package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// openViewerPane opens a file from the window's first pane and returns the
// client with the viewer pane's ID.
func openViewerPane(t *testing.T, dir, rel string, line int) (*apitest.Client, int) {
	t.Helper()
	s := newTestServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "view", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	args := map[string]string{api.ArgPath: rel}
	if line > 0 {
		args[api.ArgLine] = "2"
	}
	c.Send(&api.Command{Op: api.OpOpenFile, Target: first, Args: args})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind != api.PaneViewer {
			continue
		}
		return c, p.ID
	}
	t.Fatal("no viewer pane")
	return nil, 0
}

// TestOpenFileOpensViewer: open-file opens a viewer beside the pane and the
// viewer reads the file at the line it was given.
func TestOpenFileOpensViewer(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package pkg\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "add.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	v, pane := openViewerPane(t, dir, "pkg/add.go", 2)
	s := v.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.ViewerState
		return api.DecodeSideState(s, &st) == nil && !st.Loading && st.Path == "pkg/add.go"
	})
	var st api.ViewerState
	if err := api.DecodeSideState(s, &st); err != nil {
		t.Fatal(err)
	}
	if st.Error != "" {
		t.Fatalf("viewer error %q", st.Error)
	}
	if st.Reveal != 2 {
		t.Fatalf("reveal %d, want 2", st.Reveal)
	}
	if len(st.Lines) != 5 {
		t.Fatalf("%d lines, want 5", len(st.Lines))
	}
	if st.Lines[3].Text != "\treturn a + b" {
		t.Fatalf("line 4 %q", st.Lines[3].Text)
	}
}

// TestViewerErrorNamesRelativePath: a read failure uses the folder-relative
// path shown in the viewer header.
func TestViewerErrorNamesRelativePath(t *testing.T) {
	dir := t.TempDir()
	v, pane := openViewerPane(t, dir, "missing.go", 0)
	s := v.WaitSide(pane, func(s *api.SidePaneState) bool {
		var st api.ViewerState
		return api.DecodeSideState(s, &st) == nil && !st.Loading && st.Error != ""
	})
	var st api.ViewerState
	if err := api.DecodeSideState(s, &st); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(st.Error, "missing.go: ") || strings.Contains(st.Error, dir) {
		t.Fatalf("viewer error %q, want a relative missing.go path", st.Error)
	}
}

// TestOpenFileRejectsTraversal: open-file cannot leave the window's folder.
func TestOpenFileRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "view", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpOpenFile, Target: first, Args: map[string]string{api.ArgPath: "../x"}})
	if e := nextError(t, c); e.Code != api.CodeCommandFailed {
		t.Fatalf("got %+v, want a command failure", e)
	}
}
