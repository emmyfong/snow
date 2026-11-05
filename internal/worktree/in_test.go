package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestIn: a caller in a tracked subfolder gets that subfolder of the new
// worktree; one at the top, or in a folder git does not track, gets the
// worktree's top.
func TestIn(t *testing.T) {
	gittest.Isolate(t)
	dir := t.TempDir()
	gittest.Repo(t, dir)
	for _, sub := range []string{"cmd/snow", "scratch"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "snow", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "cmd")
	gittest.Run(t, dir, "commit", "-q", "-m", "cmd")
	ctx := context.Background()
	wt, err := Repo{Path: dir}.Create(ctx, "feat/in")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ from, want string }{
		{dir, wt.Path},
		{filepath.Join(dir, "cmd", "snow"), filepath.Join(wt.Path, "cmd", "snow")},
		{filepath.Join(dir, "scratch"), wt.Path},
	}
	for _, tt := range tests {
		got, err := Repo{Path: tt.from}.In(ctx, wt)
		if err != nil || got != tt.want {
			t.Errorf("In from %s = %q, %v; want %q", tt.from, got, err, tt.want)
		}
	}
}
