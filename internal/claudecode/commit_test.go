package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/gittest"
	"github.com/emmyfong/snow/pkg/api"
)

// TestCommitCall: the shell commands that count as running git commit.
func TestCommitCall(t *testing.T) {
	for cmd, want := range map[string]bool{
		`git commit -m "fix"`:          true,
		`cd sub && git commit -am x`:   true,
		`git -C /src commit --amend`:   true,
		"git add a\ngit commit -m b":   true,
		`git status`:                   false,
		`echo commit; git log`:         false,
		`gitk commit`:                  false,
		`npm run commit`:               false,
		`git log --grep commit | head`: true, // harmless: HEAD does not move
		`legit commit`:                 false,
		`git push origin main`:         false,
		`git commit-tree HEAD^{tree}`:  true, // harmless too
	} {
		if got := commitCall.MatchString(cmd); got != want {
			t.Errorf("%q: %v, want %v", cmd, got, want)
		}
	}
}

func hookInput(t *testing.T, name, command, cwd string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"hook_event_name": name, "tool_name": "Bash", "cwd": cwd,
		"tool_input": map[string]string{"command": command, "description": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestHookSendsCommit: around a Bash call that runs git commit, snow hook
// sends HEAD before the call, and HEAD and its parent after it.
func TestHookSendsCommit(t *testing.T) {
	repo := t.TempDir()
	gittest.Repo(t, repo)
	before := gittest.Run(t, repo, "rev-parse", "HEAD")
	path, got := fakeSnow(t, api.ProtocolVersion)
	h := Hook{Pane: "7", Socket: path, Version: "test"}
	next := func() *api.AgentState {
		t.Helper()
		select {
		case m := <-got:
			return m
		case <-time.After(5 * time.Second):
			t.Fatal("nothing sent")
			return nil
		}
	}

	if err := h.Run(strings.NewReader(hookInput(t, EventPreToolUse, `git commit -m "b"`, repo))); err != nil {
		t.Fatal(err)
	}
	if m := next(); m.Commit == nil || m.Commit.Before != before || m.Commit.Folder != repo {
		t.Fatalf("before the call: %+v", m.Commit)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repo, "add", "b.txt")
	gittest.Run(t, repo, "commit", "-q", "-m", "b")
	after := gittest.Run(t, repo, "rev-parse", "HEAD")
	if err := h.Run(strings.NewReader(hookInput(t, EventPostToolUse, `git commit -m "b"`, repo))); err != nil {
		t.Fatal(err)
	}
	if m := next(); m.Commit == nil || m.Commit.After != after || m.Commit.Parent != before {
		t.Fatalf("after the call: %+v", m.Commit)
	}

	if err := h.Run(strings.NewReader(hookInput(t, EventPostToolUse, `git status`, repo))); err != nil {
		t.Fatal(err)
	}
	if m := next(); m.Commit != nil {
		t.Errorf("a call that does not commit sent %+v", m.Commit)
	}
}

// TestReadCommitNoRepo: a folder git cannot read gives a report with no
// hashes, which marks nothing.
func TestReadCommitNoRepo(t *testing.T) {
	gittest.Isolate(t)
	e := event{Name: EventPostToolUse, Tool: "Bash", Command: "git commit -m x", Cwd: t.TempDir()}
	c := readCommit(e, time.Now().Add(time.Second))
	if c == nil || c.After != "" || c.Parent != "" {
		t.Errorf("readCommit = %+v", c)
	}
}
