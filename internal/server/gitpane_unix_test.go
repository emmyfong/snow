//go:build !windows

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// TestGitPaneBlockedGit: a git that never answers holds only its pane. The
// server goes on answering, an action is still taken, and closing the pane
// stops git.
func TestGitPaneBlockedGit(t *testing.T) {
	dir, bin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nexec sleep 600\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, dir)

	answered := make(chan struct{})
	go func() {
		_ = s.Sessions()
		close(answered)
	}()
	select {
	case <-answered:
	case <-time.After(5 * time.Second):
		t.Fatal("the server froze while git hung")
	}
	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActRefresh})
	waitGit(t, c, pane, func(st *api.GitState) bool { return st.Loading })

	start := time.Now()
	c.Send(&api.Command{Op: api.OpClose, Target: pane})
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
	s.Close()
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("closing took %v with git hung", took)
	}
}
