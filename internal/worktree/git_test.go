package worktree

import (
	"context"
	"reflect"
	"testing"
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

// TestWSLPaths: a WSL repository's paths stay Linux paths, and the exclude
// file is opened through its \\wsl.localhost path only on Windows.
func TestWSLPaths(t *testing.T) {
	r := Repo{Path: "/home/u/proj", Distro: "Ubuntu"}
	if got := r.join("/home/u/proj", ".worktrees", "feat-a"); got != "/home/u/proj/.worktrees/feat-a" {
		t.Errorf("join = %s", got)
	}
	if !r.same("/home/u/proj/", "/home/u/proj") || r.same("/home/u/Proj", "/home/u/proj") {
		t.Error("same compares WSL paths as Linux paths: exact")
	}
}
