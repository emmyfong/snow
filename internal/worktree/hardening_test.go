package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestSubmoduleRefused: a submodule has no main checkout of its own that
// git lists, so a worktree would land inside the superproject's .git. Create
// says no instead.
func TestSubmoduleRefused(t *testing.T) {
	super, sub := newRepo(t), newRepo(t)
	runGit(t, super.Path, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub.Path, "sm")
	_, err := Repo{Path: filepath.Join(super.Path, "sm")}.Create(context.Background(), "feat/a")
	if !errors.Is(err, ErrNotRepo) {
		t.Fatalf("error %v, want ErrNotRepo", err)
	}
}

// TestExcludeOnlyTopFolder: the exclude line hides .worktrees at the top of
// the repository and no folder of that name below it.
func TestExcludeOnlyTopFolder(t *testing.T) {
	r := newRepo(t)
	if _, err := r.Create(context.Background(), "feat/a"); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(r.Path, "sub", ".worktrees")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(nested, "x.txt"), "x")
	if got := runGit(t, r.Path, "status", "--porcelain"); got != "?? sub/" {
		t.Fatalf("git status:\n%s", got)
	}
}

// TestSlug: a branch name becomes a folder name every OS accepts, and
// different names that differ only by those characters meet in one folder
// (Create then refuses the second).
func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"feat/a", "feat-a"},
		{`a\b:c*d?"e<f>g|h i`, "a-b-c-d--e-f-g-h-i"},
		{"plain", "plain"},
		{"nul", "_nul"},
		{"CON", "_CON"},
		{"nul.txt", "_nul.txt"},
		{"com1", "_com1"},
		{"lpt9.x", "_lpt9.x"},
		{"com10", "com10"},
		{"console", "console"},
		{"nulls", "nulls"},
		{"feat/nul", "feat-nul"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestStderrText: git's own words are UTF-8, and wsl.exe's own errors are
// UTF-16 with NULs, which come out as text either way.
func TestStderrText(t *testing.T) {
	utf16 := []byte{'n', 0, 'o', 0, ' ', 0, 'd', 0, 'i', 0, 's', 0, 't', 0, 'r', 0, 'o', 0}
	for _, tt := range []struct {
		name string
		in   []byte
		wsl  bool
		want string
	}{
		{"git", []byte("fatal: x\n"), false, "fatal: x"},
		{"git in a distribution", []byte("fatal: x\n"), true, "fatal: x"},
		{"wsl.exe error", utf16, true, "no distro"},
		{"a native repo never decodes", []byte("a\x00b"), false, "a\x00b"},
	} {
		if got := stderrText(tt.in, tt.wsl); got != tt.want {
			t.Errorf("%s: stderrText = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestClassifyWSL: exit code 127 from env is git not found in the
// distribution, whatever language it says so in.
func TestClassifyWSL(t *testing.T) {
	wsl := Repo{Path: "/p", Distro: "Ubuntu"}
	for _, stderr := range []string{"env: 'git': No such file or directory", "env : 'git' : Datei oder Verzeichnis nicht gefunden"} {
		err := wsl.classify(&failure{sub: "worktree", code: 127, stderr: stderr, cause: errors.New("exit status 127")})
		if !errors.Is(err, ErrNoGit) {
			t.Errorf("stderr %q: error %v, want ErrNoGit", stderr, err)
		}
	}
	native := Repo{Path: "/p"}
	if err := native.classify(&failure{sub: "x", code: 127, stderr: "x", cause: errors.New("x")}); errors.Is(err, ErrNoGit) {
		t.Error("a native 127 is not ErrNoGit: exec.ErrNotFound says that")
	}
}
