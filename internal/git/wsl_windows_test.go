package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

// TestGitPaneWSL: in a repository inside a WSL distribution, status lists
// changes with Linux paths, and a file stages and shows its diff. It skips
// when no distribution has git, as on a CI runner.
func TestGitPaneWSL(t *testing.T) {
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
			if _, err := r.Run(ctx, dir, args...); err != nil {
				t.Fatal(err)
			}
		}
		if err := exec.Command("wsl.exe", "-d", p.Distro, "--cd", dir, "--exec", "sh", "-c", "mkdir sub && echo hi > 'sub/a b.txt'").Run(); err != nil {
			t.Fatal(err)
		}
		st, err := r.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Branch != "main" || len(st.Changes) != 1 || st.Changes[0] != (Change{Path: "sub/a b.txt", Work: "?"}) {
			t.Fatalf("status %+v", st)
		}
		if err := r.Stage(ctx, "sub/a b.txt"); err != nil {
			t.Fatal(err)
		}
		d, err := r.Diff(ctx, Change{Path: "sub/a b.txt", Index: "A"}, true)
		if err != nil || len(d.Lines) != 2 || d.Lines[1] != (Line{LineAdd, "hi"}) {
			t.Fatalf("diff %+v, %v", d, err)
		}
		return
	}
	t.Skip("no WSL distribution with git")
}
