package worktree

import (
	"runtime"
	"testing"

	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/internal/shell"
)

// TestWSLPaths: a WSL repository's paths stay Linux paths when Snow joins
// and compares them.
func TestWSLPaths(t *testing.T) {
	r := Repo{Path: "/home/u/proj", Distro: "Ubuntu"}
	if got := r.join("/home/u/proj", ".worktrees", "feat-a"); got != "/home/u/proj/.worktrees/feat-a" {
		t.Errorf("join = %s", got)
	}
	if !r.same("/home/u/proj/", "/home/u/proj") || r.same("/home/u/Proj", "/home/u/proj") {
		t.Error("same compares WSL paths as Linux paths: exact")
	}
}

// TestOSPath: a WSL repository's files are opened through their
// \\wsl.localhost path on Windows, which is the only OS that has one, and
// by their own path anywhere else. A native repository's path never changes.
func TestOSPath(t *testing.T) {
	wsl := Repo{Path: "/home/u/p", Distro: "Ubuntu"}
	want := "/home/u/p/.git/info/exclude"
	if runtime.GOOS == "windows" {
		want = shell.WSLToUNC("Ubuntu", want)
	}
	if got := wsl.osPath("/home/u/p/.git/info/exclude"); got != want {
		t.Errorf("WSL osPath = %q, want %q", got, want)
	}
	native := Repo{Path: "x"}
	if got := native.osPath("some/path"); got != "some/path" {
		t.Errorf("native osPath = %q", got)
	}
}

// TestBranchTaken: both ways git says another caller made the branch first
// count; other failures do not.
func TestBranchTaken(t *testing.T) {
	tests := []struct {
		stderr string
		want   bool
	}{
		{"Preparing worktree (new branch 'agent/1')\nfatal: cannot lock ref 'refs/heads/agent/1': reference already exists", true},
		{"fatal: a branch named 'agent/1' already exists", true},
		{"fatal: '/src/.worktrees/agent-1' already exists", false},
		{"fatal: not a git repository", false},
	}
	for _, tt := range tests {
		if got := branchTaken(git.NewError("worktree", 255, tt.stderr, nil)); got != tt.want {
			t.Errorf("branchTaken(%q) = %v, want %v", tt.stderr, got, tt.want)
		}
	}
}
