// Package git runs the git command for Snow: as the user would run it, and
// in the repository's own distribution for a WSL folder. It reads status,
// diffs, history, and stages and commits files. It never writes git config
// and never adds to a commit message.
//
// The functions block while git runs. A caller on the server's goroutine
// must run them somewhere else.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// The failures a caller can act on. Each comes wrapped with git's own
// message, so test for one with errors.Is.
var (
	// ErrNoGit means git is not installed, or not in the distribution.
	ErrNoGit = errors.New("git is not installed")
	// ErrNotRepo means the folder is not in a git repository, or the
	// repository has no checkout.
	ErrNotRepo = errors.New("not a git repository")
)

// Repo is a git repository: a folder in it, and the WSL distribution it
// belongs to, if any. Path may be any folder of the repository, in the
// repository's own form: a Linux path for WSL.
type Repo struct {
	Path   string
	Distro string // empty for a repository of the OS Snow runs on
}

// gitEnv keeps git from asking questions no one can answer, and keeps its
// messages in English, which Classify reads.
var gitEnv = []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}

// maxOutput bounds what Run keeps of git's output. A diff of a huge file
// must not fill the server's memory; the parsers cut far below this.
const maxOutput = 16 << 20

// Command builds the git process for args, started in dir. A WSL
// repository runs through wsl.exe, which does not pass the environment on,
// so env sets it inside the distribution.
func (r Repo) Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
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

// Error is a git command that did not succeed.
type Error struct {
	Sub    string // the git subcommand
	Code   int    // the exit code, or -1 if git did not run
	Stderr string
	cause  error
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		return "git " + e.Sub + ": " + e.Stderr
	}
	return "git " + e.Sub + ": " + e.cause.Error()
}

func (e *Error) Unwrap() error { return e.cause }

// NewError returns the Error of a git subcommand that exited with code and
// wrote stderr, for code that tells failures apart.
func NewError(sub string, code int, stderr string, cause error) *Error {
	return &Error{Sub: sub, Code: code, Stderr: stderr, cause: cause}
}

// Run runs git in dir and returns what it wrote to its standard output.
// A failure comes back as an *Error; Classify turns it into a typed error.
func (r Repo) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return r.RunInput(ctx, dir, nil, args...)
}

// RunInput is Run with in as git's standard input.
func (r Repo) RunInput(ctx context.Context, dir string, in io.Reader, args ...string) (string, error) {
	cmd := r.Command(ctx, dir, args...)
	stdout := &capped{max: maxOutput}
	var stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, stdout, &stderr
	if err := cmd.Run(); err != nil {
		e := &Error{Sub: args[0], Code: -1, Stderr: stderrText(stderr.Bytes(), r.Distro != ""), cause: err}
		if ctxErr := ctx.Err(); ctxErr != nil {
			// A killed git says "signal: killed"; the caller wants to know why.
			e.cause = fmt.Errorf("%w: %w", ctxErr, err)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			e.Code = exit.ExitCode()
		}
		return stdout.String(), e
	}
	return stdout.String(), nil
}

// capped keeps the first max bytes written to it and drops the rest, so
// git still runs to its end.
type capped struct {
	bytes.Buffer
	max int
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.max - c.Len(); room > 0 {
		c.Buffer.Write(p[:min(room, len(p))])
	}
	return len(p), nil
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

// Declined reports whether err is git answering "no" to a yes-or-no
// command, which it does with exit code 1, and not git failing to answer.
func Declined(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == 1
}

// Classify gives a failure that says what is wrong with the repository or
// with git itself a typed error. Any other failure passes through.
func (r Repo) Classify(err error) error {
	var e *Error
	if !errors.As(err, &e) {
		return err
	}
	switch {
	case errors.Is(e.cause, exec.ErrNotFound),
		// env exits 127 when it cannot find git. Its message is in the
		// distribution's language, so only the code is read.
		r.Distro != "" && e.Code == 127:
		return fmt.Errorf("%w: %w", ErrNoGit, err)
	case e.Code == 128 && (strings.Contains(e.Stderr, "not a git repository") || strings.Contains(e.Stderr, "cannot change to")):
		return fmt.Errorf("%w: %w", ErrNotRepo, err)
	}
	return err
}

// pathspec names path, relative to the top of the repository, as git must
// read it: from the top whatever folder git runs in, and with no wildcards.
func pathspec(path string) string { return ":(top,literal)" + path }
