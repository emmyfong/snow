package worktree

import (
	"context"
	"path/filepath"
	"strings"
)

// Worktree is one checkout of a repository.
type Worktree struct {
	Path     string // in the repository's own form
	Branch   string // short name; empty for a detached HEAD
	Head     string // the commit checked out
	Main     bool   // the repository's own checkout, not a linked worktree
	Detached bool
	Locked   bool
	Bare     bool // a bare repository has no checkout
}

// List returns the worktrees of the repository, the main checkout first.
func (r Repo) List(ctx context.Context) ([]Worktree, error) {
	out, err := r.git(ctx, r.Path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, r.classify(err)
	}
	wts := parseList(out)
	if r.Distro == "" {
		// git writes forward slashes on Windows too.
		for i := range wts {
			wts[i].Path = filepath.FromSlash(wts[i].Path)
		}
	}
	return wts, nil
}

// parseList reads the output of git worktree list --porcelain: one block
// of lines per worktree, split by an empty line.
func parseList(out string) []Worktree {
	var wts []Worktree
	var cur *Worktree
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			wts = append(wts, Worktree{Path: val, Main: len(wts) == 0})
			cur = &wts[len(wts)-1]
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		}
	}
	return wts
}
