// Package worktree creates, lists, and removes the git worktrees that give
// each coding agent a folder of its own. Git runs through the git command,
// as the user would run it, in the repository's own distribution for a WSL
// folder. Snow never writes git config. It needs git 2.17 or later, the
// first version with `git worktree remove`.
//
// The functions block while git runs. A caller on the server's goroutine
// must run them somewhere else.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// Repo is a git repository: a folder in it, and the WSL distribution it
// belongs to, if any. Path may be any folder of the repository, in the
// repository's own form: a Linux path for WSL.
type Repo struct {
	Path   string
	Distro string // empty for a repository of the OS Snow runs on
}

// gitEnv keeps git from asking questions no one can answer, and keeps its
// messages in English, which classify reads for the repository errors.
var gitEnv = []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}

// command builds the git process for args, started in dir. A WSL
// repository runs through wsl.exe, which does not pass the environment on,
// so env sets it inside the distribution.
func (r Repo) command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if r.Distro == "" {
		cmd = exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), gitEnv...)
	} else {
		wsl := []string{"-d", r.Distro, "--cd", dir, "--exec", "env"}
		wsl = append(append(append(wsl, gitEnv...), "git"), args...)
		cmd = exec.CommandContext(ctx, "wsl.exe", wsl...)
	}
	// A child of git, such as a hook, can hold the output pipes after git
	// is killed. Run gives up on them after this long.
	cmd.WaitDelay = 2 * time.Second
	shell.HideWindow(cmd)
	return cmd
}

// failure is a git command that did not succeed.
type failure struct {
	sub    string // the git subcommand
	code   int    // the exit code, or -1 if git did not run
	stderr string
	cause  error
}

func (f *failure) Error() string {
	if f.stderr != "" {
		return "git " + f.sub + ": " + f.stderr
	}
	return "git " + f.sub + ": " + f.cause.Error()
}

func (f *failure) Unwrap() error { return f.cause }

// git runs git in dir and returns what it wrote to its standard output.
// A failure comes back as a *failure; classify turns it into a typed error.
func (r Repo) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := r.command(ctx, dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		f := &failure{sub: args[0], code: -1, stderr: stderrText(stderr.Bytes(), r.Distro != ""), cause: err}
		if ctxErr := ctx.Err(); ctxErr != nil {
			// A killed git says "signal: killed"; the caller wants to know why.
			f.cause = fmt.Errorf("%w: %w", ctxErr, err)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			f.code = exit.ExitCode()
		}
		return "", f
	}
	return stdout.String(), nil
}

// stderrText returns what a command wrote to its error output as text. Git
// writes UTF-8. wsl.exe writes its own errors, such as a distribution that
// is not there, in UTF-16 with NULs between the letters, so for WSL a NUL
// means UTF-16.
func stderrText(b []byte, wsl bool) string {
	if wsl && bytes.IndexByte(b, 0) >= 0 {
		return strings.TrimSpace(shell.DecodeUTF16LE(b))
	}
	return strings.TrimSpace(string(b))
}

// declined reports whether err is git answering "no" to a yes-or-no command,
// which it does with exit code 1, and not git failing to answer.
func declined(err error) bool {
	var f *failure
	return errors.As(err, &f) && f.code == 1
}

// classify gives a failure that says what is wrong with the repository or
// with git itself a typed error. Any other failure passes through.
func (r Repo) classify(err error) error {
	var f *failure
	if !errors.As(err, &f) {
		return err
	}
	switch {
	case errors.Is(f.cause, exec.ErrNotFound),
		// env exits 127 when it cannot find git. Its message is in the
		// distribution's language, so only the code is read.
		r.Distro != "" && f.code == 127:
		return fmt.Errorf("%w: %w", ErrNoGit, err)
	case f.code == 128 && (strings.Contains(f.stderr, "not a git repository") || strings.Contains(f.stderr, "cannot change to")):
		return fmt.Errorf("%w: %w", ErrNotRepo, err)
	}
	return err
}

// join joins path elements in the repository's own form.
func (r Repo) join(elem ...string) string {
	if r.Distro == "" {
		return filepath.Join(elem...)
	}
	return path.Join(elem...)
}

// dir returns the folder that holds p, in the repository's own form.
func (r Repo) dir(p string) string {
	if r.Distro == "" {
		return filepath.Dir(p)
	}
	return path.Dir(p)
}

// same reports whether a and b are one path. git lists real paths, so a
// path through a symlink, or a Windows short name, is resolved first. A WSL
// path cannot be resolved from here, so the caller passes the path that
// Create or List returned. Windows and macOS file systems ignore case.
func (r Repo) same(a, b string) bool {
	if r.Distro != "" {
		return path.Clean(a) == path.Clean(b)
	}
	a, b = resolve(a), resolve(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// resolve returns the real path of p, or its clean form when p is gone.
func resolve(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// osPath is where Snow opens a path of the repository: the path, or for a
// WSL repository on Windows, its \\wsl.localhost path.
func (r Repo) osPath(p string) string {
	if r.Distro != "" && runtime.GOOS == "windows" {
		return shell.WSLToUNC(r.Distro, p)
	}
	return p
}
