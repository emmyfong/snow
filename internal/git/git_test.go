package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestCommand: a WSL repository runs git inside its distribution through
// wsl.exe, with the environment set there, and a native one runs git itself.
func TestCommand(t *testing.T) {
	ctx := context.Background()
	wsl := Repo{Path: "/home/u/proj", Distro: "Ubuntu-24.04"}.Command(ctx, "/home/u/proj", "worktree", "list", "--porcelain")
	wantWSL := []string{"wsl.exe", "-d", "Ubuntu-24.04", "--cd", "/home/u/proj", "--exec", "env", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0", "git", "worktree", "list", "--porcelain"}
	if !reflect.DeepEqual(wsl.Args, wantWSL) {
		t.Errorf("WSL args %q, want %q", wsl.Args, wantWSL)
	}
	native := Repo{Path: "/p"}.Command(ctx, "/p", "status")
	if want := []string{"git", "-C", "/p", "status"}; !reflect.DeepEqual(native.Args, want) {
		t.Errorf("native args %q, want %q", native.Args, want)
	}
	if !containsAll(native.Env, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0") {
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
		err := wsl.Classify(NewError("worktree", 127, stderr, errors.New("exit status 127")))
		if !errors.Is(err, ErrNoGit) {
			t.Errorf("stderr %q: error %v, want ErrNoGit", stderr, err)
		}
	}
	native := Repo{Path: "/p"}
	if err := native.Classify(NewError("x", 127, "x", errors.New("x"))); errors.Is(err, ErrNoGit) {
		t.Error("a native 127 is not ErrNoGit: exec.ErrNotFound says that")
	}
}

// TestDeclined: only git's exit code 1 is an answer of "no" to a question.
// A crash, a kill, or a failure to start is a failure, and no sentinel
// error may hide it.
func TestDeclined(t *testing.T) {
	exit := func(code int) error {
		cmd := exec.Command("git", "check-ref-format", "refs/heads/bad..name")
		if code != 1 {
			cmd = exec.Command("git", "--no-such-option")
		}
		err := cmd.Run()
		var e *exec.ExitError
		if !errors.As(err, &e) {
			t.Fatalf("git did not fail: %v", err)
		}
		return NewError("x", e.ExitCode(), "", err)
	}
	if !Declined(exit(1)) {
		t.Error("exit code 1 is a no")
	}
	if Declined(exit(129)) {
		t.Error("exit code 129 is a failure")
	}
	if Declined(NewError("x", -1, "", context.Canceled)) || Declined(fmt.Errorf("plain")) {
		t.Error("a kill or a plain error is a failure")
	}
}

// TestCapped: output past the cap is dropped, and the writer never fails,
// so git is not killed by a full pipe.
func TestCapped(t *testing.T) {
	c := &capped{max: 5}
	for _, s := range []string{"abc", "defg", "hij"} {
		if n, err := c.Write([]byte(s)); n != len(s) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	if got := c.String(); got != "abcde" {
		t.Errorf("kept %q, want abcde", got)
	}
}

// TestNotRepo: a folder outside any repository reads as ErrNotRepo.
func TestNotRepo(t *testing.T) {
	gittest.Isolate(t)
	_, err := Repo{Path: t.TempDir()}.Status(context.Background())
	if !errors.Is(err, ErrNotRepo) {
		t.Fatalf("Status in a plain folder: %v, want ErrNotRepo", err)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q lacks git's own message", err)
	}
}

// TestErrorNoCause: an Error with neither stderr nor a cause still reads.
func TestErrorNoCause(t *testing.T) {
	if got := NewError("status", 1, "", nil).Error(); got != "git status failed" {
		t.Errorf("Error() = %q", got)
	}
}
