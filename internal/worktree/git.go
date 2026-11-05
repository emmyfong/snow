// Package worktree creates, lists, and removes the git worktrees that give
// each coding agent a folder of its own. Git runs through internal/git, as
// the user would run it, in the repository's own distribution for a WSL
// folder. Snow never writes git config. It needs git 2.17 or later, the
// first version with `git worktree remove`.
//
// The functions block while git runs. A caller on the server's goroutine
// must run them somewhere else.
package worktree

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/internal/shell"
)

// Repo is a git repository: a folder in it, and the WSL distribution it
// belongs to, if any. Path may be any folder of the repository, in the
// repository's own form: a Linux path for WSL.
type Repo struct {
	Path   string
	Distro string // empty for a repository of the OS Snow runs on
}

// git runs git in dir and returns what it wrote to its standard output.
// A failure comes back as a *git.Error; classify turns it into a typed error.
func (r Repo) git(ctx context.Context, dir string, args ...string) (string, error) {
	return git.Repo(r).Run(ctx, dir, args...)
}

// declined reports whether err is git answering "no" to a yes-or-no command.
func declined(err error) bool { return git.Declined(err) }

// branchTaken reports whether err is git refusing to make a branch that
// already exists. git runs with LC_ALL=C, so its messages are English.
func branchTaken(err error) bool {
	var e *git.Error
	return errors.As(err, &e) &&
		(strings.Contains(e.Stderr, "reference already exists") ||
			strings.Contains(e.Stderr, "a branch named") && strings.Contains(e.Stderr, "already exists"))
}

// classify gives a failure that says what is wrong with the repository or
// with git itself a typed error. Any other failure passes through.
func (r Repo) classify(err error) error { return git.Repo(r).Classify(err) }

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
