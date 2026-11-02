package worktree

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

// TestCommand: a WSL repository runs git inside its distribution through
// wsl.exe, with the environment set there, and a native one runs git itself.
func TestCommand(t *testing.T) {
	ctx := context.Background()
	wsl := Repo{Path: "/home/u/proj", Distro: "Ubuntu-24.04"}.command(ctx, "/home/u/proj", "worktree", "list", "--porcelain")
	wantWSL := []string{"wsl.exe", "-d", "Ubuntu-24.04", "--cd", "/home/u/proj", "--exec", "env", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "git", "worktree", "list", "--porcelain"}
	if !reflect.DeepEqual(wsl.Args, wantWSL) {
		t.Errorf("WSL args %q, want %q", wsl.Args, wantWSL)
	}
	native := Repo{Path: "/p"}.command(ctx, "/p", "status")
	if want := []string{"git", "-C", "/p", "status"}; !reflect.DeepEqual(native.Args, want) {
		t.Errorf("native args %q, want %q", native.Args, want)
	}
	if !containsAll(native.Env, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C") {
		t.Errorf("native env lacks the git settings")
	}
}

func containsAll(env []string, want ...string) bool {
	have := map[string]bool{}
	for _, e := range env {
		have[e] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

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
