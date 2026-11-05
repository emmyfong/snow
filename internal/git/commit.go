package git

import (
	"context"
	"errors"
	"strings"
)

// ErrEmptyMessage means a commit was asked for with no message.
var ErrEmptyMessage = errors.New("the commit message is empty")

// Stage stages the changes to paths, which are from the top of the
// repository. A deleted file's deletion is staged too.
func (r Repo) Stage(ctx context.Context, paths ...string) error {
	return r.onPaths(ctx, []string{"add", "--all", "--"}, paths)
}

// StageAll stages every change in the repository, also those the pane
// does not list, with one short command line.
func (r Repo) StageAll(ctx context.Context) error {
	if _, err := r.Run(ctx, r.Path, "add", "--all", "--", ":/"); err != nil {
		return r.Classify(err)
	}
	return nil
}

// Unstage takes the staged changes to paths back out of the index. The
// files keep their changes. A rename needs both its paths.
func (r Repo) Unstage(ctx context.Context, paths ...string) error {
	// reset, not restore --staged: it also works on a branch with no commit.
	return r.onPaths(ctx, []string{"reset", "--quiet", "--"}, paths)
}

func (r Repo) onPaths(ctx context.Context, args, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for _, p := range paths {
		args = append(args, pathspec(p))
	}
	if _, err := r.Run(ctx, r.Path, args...); err != nil {
		return r.Classify(err)
	}
	return nil
}

// Commit commits what is staged with message, as the user's own git
// identity, and returns the new commit's short hash. Git's hooks run as
// they would for the user. A git with no identity fails with git's own
// message.
func (r Repo) Commit(ctx context.Context, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", ErrEmptyMessage
	}
	// The message goes on standard input, so no length limit or leading
	// dash in it can change the command line.
	if _, err := r.RunInput(ctx, r.Path, strings.NewReader(message), "commit", "--file=-"); err != nil {
		return "", r.Classify(err)
	}
	head, err := r.Run(ctx, r.Path, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", r.Classify(err)
	}
	return strings.TrimSpace(head), nil
}
