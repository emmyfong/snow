package files

import (
	"context"
	"errors"
	"strings"

	"github.com/emmyfong/snow/internal/git"
)

// GitMarks returns git's one-letter change for each path below root, with
// paths from root. A folder in no repository, or without git, has no
// marks and no error.
func GitMarks(ctx context.Context, root, distro string) (map[string]string, error) {
	repo := git.Repo{Path: root, Distro: distro}
	prefix, err := repo.Prefix(ctx)
	switch {
	case errors.Is(err, git.ErrNotRepo), errors.Is(err, git.ErrNoGit):
		return nil, nil
	case err != nil:
		return nil, err
	}
	st, err := repo.Status(ctx)
	switch {
	case errors.Is(err, git.ErrNotRepo), errors.Is(err, git.ErrNoGit):
		return nil, nil
	case err != nil:
		return nil, err
	}
	out := make(map[string]string, len(st.Changes))
	for _, c := range st.Changes {
		if p, ok := below(prefix, c.Path); ok {
			if m := mark(c); m != "" {
				out[p] = m
			}
		}
		if c.Orig != "" {
			if p, ok := below(prefix, c.Orig); ok {
				out[p] = "D"
			}
		}
	}
	return out, nil
}

// below returns p without prefix, which ends in a slash or is empty, and
// whether p is under it.
func below(prefix, p string) (string, bool) {
	if prefix == "" {
		return p, true
	}
	rest, ok := strings.CutPrefix(p, prefix)
	return rest, ok
}

// mark is git's one-letter change for c. A file with both a staged and an
// unstaged change shows the unstaged one, the one the file has now.
func mark(c git.Change) string {
	switch {
	case c.Conflict:
		return "U"
	case c.Work != "" && c.Work != " ":
		return c.Work
	case c.Index != "":
		return c.Index
	}
	return ""
}

// Parent is the folder of p in the tree, or "" at the root.
func Parent(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}
