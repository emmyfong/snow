package worktree

import (
	"context"
	"os"
	"strings"
)

// In returns the folder of wt that matches r.Path: r.Path's place below
// the top of its own checkout, joined onto wt.Path. A folder that wt does
// not have, such as one git does not track, gives wt.Path.
func (r Repo) In(ctx context.Context, wt Worktree) (string, error) {
	prefix, err := r.git(ctx, r.Path, "rev-parse", "--show-prefix")
	if err != nil {
		return "", r.classify(err)
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return wt.Path, nil
	}
	dir := r.join(append([]string{wt.Path}, strings.Split(prefix, "/")...)...)
	if fi, err := os.Stat(r.osPath(dir)); err != nil || !fi.IsDir() {
		return wt.Path, nil
	}
	return dir, nil
}
