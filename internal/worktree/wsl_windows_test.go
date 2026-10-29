package worktree

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

// wslRepo makes a repository with one commit inside a WSL distribution
// that has git, and removes it when the test ends. It skips the test when
// no distribution has git, as on a CI runner.
func wslRepo(t *testing.T) Repo {
	t.Helper()
	for _, p := range shell.Detect() {
		if p.Kind != shell.KindWSL || p.Distro == "" {
			continue
		}
		if exec.Command("wsl.exe", "-d", p.Distro, "--exec", "git", "--version").Run() != nil {
			continue
		}
		out, err := exec.Command("wsl.exe", "-d", p.Distro, "--exec", "mktemp", "-d").Output()
		if err != nil {
			continue
		}
		dir := strings.TrimSpace(string(out))
		t.Cleanup(func() { _ = exec.Command("wsl.exe", "-d", p.Distro, "--exec", "rm", "-rf", dir).Run() })
		r := Repo{Path: dir, Distro: p.Distro}
		ctx := context.Background()
		for _, args := range [][]string{
			{"init", "-q", "-b", "main"},
			{"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-q", "--allow-empty", "-m", "first"},
		} {
			if _, err := r.git(ctx, dir, args...); err != nil {
				t.Fatal(err)
			}
		}
		return r
	}
	t.Skip("no WSL distribution with git")
	return Repo{}
}

// TestWSLRepo: a worktree for a repository inside a WSL distribution is made,
// listed, kept out of git status, and removed, with Linux paths throughout.
func TestWSLRepo(t *testing.T) {
	r := wslRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "feat/wsl")
	if err != nil {
		t.Fatal(err)
	}
	if want := r.Path + "/.worktrees/feat-wsl"; wt.Path != want {
		t.Fatalf("path %s, want %s", wt.Path, want)
	}
	list, err := r.List(ctx)
	if err != nil || len(list) != 2 || list[1].Path != wt.Path || list[1].Branch != "feat/wsl" {
		t.Fatalf("list %+v, %v", list, err)
	}
	if out, err := r.git(ctx, r.Path, "status", "--porcelain"); err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("the worktree shows in git status: %q, %v", out, err)
	}
	if err := r.Remove(ctx, wt.Path); err != nil {
		t.Fatal(err)
	}
}
