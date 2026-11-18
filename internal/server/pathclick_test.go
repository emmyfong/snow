package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestPathClickOpensFile: the open-file command a click on a path in a chat
// reply sends opens the viewer at that path and line.
func TestPathClickOpensFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package pkg\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "add.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work", Create: true, Dir: dir})
	chat := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	c.WaitChat(chat, func(*api.ChatSnapshot) bool { return true })
	c.Send(&api.Command{Op: api.OpOpenFile, Target: chat, Args: map[string]string{
		api.ArgPath: "pkg/add.go", api.ArgLine: "2",
	}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	viewer := 0
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneViewer {
			viewer = p.ID
		}
	}
	if viewer == 0 {
		t.Fatal("open-file did not open a viewer")
	}
	state := c.WaitSide(viewer, func(s *api.SidePaneState) bool {
		var st api.ViewerState
		return api.DecodeSideState(s, &st) == nil && !st.Loading && st.Path == "pkg/add.go"
	})
	var st api.ViewerState
	if err := api.DecodeSideState(state, &st); err != nil {
		t.Fatal(err)
	}
	if st.Error != "" {
		t.Fatalf("viewer error %q", st.Error)
	}
	if st.Reveal != 2 {
		t.Fatalf("reveal %d, want 2", st.Reveal)
	}
}
