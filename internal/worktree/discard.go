package worktree

import (
	"context"
	"errors"
)

// Discard takes back a worktree that Create made for work that never
// started: it removes wt as Remove does, then deletes wt's branch if the
// branch still points at wt.Head, so no commit is lost and the next
// Create with that name is not refused. A worktree that Remove refuses is
// kept, with its branch.
func (r Repo) Discard(ctx context.Context, wt Worktree) error {
	if err := r.Remove(ctx, wt.Path); err != nil {
		return err
	}
	main, err := r.mainCheckout(ctx)
	if err != nil {
		return err
	}
	if wt.Branch == "" {
		return errors.New("worktree: no branch to discard")
	}
	return r.undoBranch(ctx, main.Path, wt.Branch, wt.Head)
}
