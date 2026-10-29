package worktree

import "errors"

// The failures a caller can act on. Each comes wrapped with git's own
// message, so test for one with errors.Is.
var (
	// ErrNoGit means git is not installed, or not in the distribution.
	ErrNoGit = errors.New("git is not installed")
	// ErrNotRepo means the folder is not in a git repository, or the
	// repository has no checkout.
	ErrNotRepo = errors.New("not a git repository")
	// ErrNoCommits means the repository has no commit to branch from.
	ErrNoCommits = errors.New("the repository has no commits")
	// ErrBadBranch means git does not accept the branch name.
	ErrBadBranch = errors.New("not a valid branch name")
	// ErrBranchExists means the branch is already there. Snow never takes
	// over a branch that holds someone's work.
	ErrBranchExists = errors.New("the branch already exists")
	// ErrPathExists means the folder for the worktree is already there.
	ErrPathExists = errors.New("the worktree folder already exists")
	// ErrDirty means the worktree has changes that are not committed, or
	// files git does not track. Snow never removes such a worktree.
	ErrDirty = errors.New("the worktree has uncommitted changes")
	// ErrNotWorktree means the path is not a worktree of the repository.
	ErrNotWorktree = errors.New("not a worktree of this repository")
	// ErrMainWorktree means the path is the main checkout, which Snow
	// never removes.
	ErrMainWorktree = errors.New("the main checkout is not a worktree to remove")
)
