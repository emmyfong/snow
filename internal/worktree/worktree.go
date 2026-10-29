package worktree

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// dirName is the folder of the repository that holds the worktrees.
const dirName = ".worktrees"

// Create makes a worktree for branch, a new branch that starts at the
// current commit of the repository, in <repository>/.worktrees/<branch>
// with each / in the name as a dash. The main checkout, its index, and its
// branch stay as they are, and the repository config is not written.
//
// The repository is the main checkout, also when r.Path is in a linked
// worktree or in a folder below the top. The returned path is the new
// worktree's top, so a caller that works in a subfolder joins it on.
func (r Repo) Create(ctx context.Context, branch string) (Worktree, error) {
	main, err := r.mainCheckout(ctx)
	if err != nil {
		return Worktree{}, err
	}
	if _, err := r.git(ctx, main.Path, "check-ref-format", "--branch", branch); err != nil {
		return Worktree{}, fmt.Errorf("%w: %q", ErrBadBranch, branch)
	}
	head, err := r.git(ctx, main.Path, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return Worktree{}, fmt.Errorf("%w: %w", ErrNoCommits, err)
	}
	if _, err := r.git(ctx, main.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return Worktree{}, fmt.Errorf("%w: %s", ErrBranchExists, branch)
	}
	if err := r.ensureExcluded(ctx, main.Path); err != nil {
		return Worktree{}, err
	}
	dir := r.join(main.Path, dirName, slug(branch))
	if _, err := r.git(ctx, main.Path, "worktree", "add", "-b", branch, dir); err != nil {
		var f *failure
		if errors.As(err, &f) && strings.Contains(f.stderr, "already exists") {
			return Worktree{}, fmt.Errorf("%w: %w", ErrPathExists, err)
		}
		return Worktree{}, r.classify(err)
	}
	return Worktree{Path: dir, Branch: branch, Head: strings.TrimSpace(head)}, nil
}

// Remove removes the worktree at path, and keeps its branch. It refuses a
// worktree with uncommitted changes or untracked files, so no work is lost,
// and the main checkout. Ignored files do not count: they come back with a
// build.
func (r Repo) Remove(ctx context.Context, path string) error {
	list, err := r.List(ctx)
	if err != nil {
		return err
	}
	var target *Worktree
	for i := range list {
		if r.same(list[i].Path, path) {
			target = &list[i]
		}
	}
	switch {
	case target == nil:
		return fmt.Errorf("%w: %s", ErrNotWorktree, path)
	case target.Main:
		return fmt.Errorf("%w: %s", ErrMainWorktree, path)
	}
	status, err := r.git(ctx, target.Path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return r.classify(err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%w: %s", ErrDirty, target.Path)
	}
	// From the main checkout: the worktree's own folder is about to go.
	if _, err := r.git(ctx, list[0].Path, "worktree", "remove", target.Path); err != nil {
		return r.classify(err)
	}
	return nil
}

// mainCheckout returns the repository's own checkout.
func (r Repo) mainCheckout(ctx context.Context) (Worktree, error) {
	list, err := r.List(ctx)
	if err != nil {
		return Worktree{}, err
	}
	if len(list) == 0 || list[0].Bare {
		return Worktree{}, fmt.Errorf("%w: %s has no checkout", ErrNotRepo, r.Path)
	}
	return list[0], nil
}

// slug turns a branch name into a folder name. Git allows names a Windows
// folder does not, such as a slash or a colon.
func slug(branch string) string {
	return strings.Map(func(c rune) rune {
		if strings.ContainsRune(`/\:*?"<>| `, c) {
			return '-'
		}
		return c
	}, branch)
}
