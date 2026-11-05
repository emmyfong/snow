package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/git"
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
	if m := next(); m.Commit == nil || m.Commit.Phase != api.CommitBefore || m.Commit.Before != before {
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
	if m := next(); m.Commit == nil || m.Commit.Phase != api.CommitAfter || m.Commit.After != after || m.Commit.Parent != before {
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
	c := readCommit(e, "", time.Now().Add(time.Second))
	if c == nil || c.Phase != api.CommitAfter || c.After != "" || c.Parent != "" {
		t.Errorf("readCommit = %+v", c)
	}
}

// TestHeadRepo: the Windows snow.exe that a Claude Code in WSL starts
// reads git inside that distribution; a Windows folder, or any folder on
// another OS, reads it directly.
func TestHeadRepo(t *testing.T) {
	tests := []struct {
		goos, cwd, distro string
		want              git.Repo
	}{
		{"windows", "/home/u/p", "Ubuntu", git.Repo{Path: "/home/u/p", Distro: "Ubuntu"}},
		{"windows", `C:\src\p`, "Ubuntu", git.Repo{Path: `C:\src\p`}},
		{"windows", "/home/u/p", "", git.Repo{Path: "/home/u/p"}},
		{"linux", "/home/u/p", "Ubuntu", git.Repo{Path: "/home/u/p"}},
	}
	for _, tt := range tests {
		if got := headRepo(tt.goos, tt.cwd, tt.distro); got != tt.want {
			t.Errorf("headRepo(%s, %s, %s) = %+v, want %+v", tt.goos, tt.cwd, tt.distro, got, tt.want)
		}
	}
}
