package worktree

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ensureExcluded keeps the worktrees out of the user's git status with a
// line in .git/info/exclude. That file is local to the repository and no
// part of git's config or of a commit. The file is shared by every
// worktree, so one write covers them all.
func (r Repo) ensureExcluded(ctx context.Context, main string) error {
	out, err := r.git(ctx, main, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return r.classify(err)
	}
	file := strings.TrimRight(out, "\r\n")
	if r.Distro != "" {
		if !path.IsAbs(file) {
			file = path.Join(main, file)
		}
	} else {
		file = filepath.FromSlash(file)
		if !filepath.IsAbs(file) {
			file = filepath.Join(main, file)
		}
	}
	return addExclude(r.osPath(file))
}

// addExclude appends the worktree folder to the exclude file, unless a line
// there already names it.
func addExclude(file string) (err error) {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		// git ignores spaces at the end of a pattern, and not at the start.
		switch strings.TrimRight(line, " \t\r") {
		case dirName, dirName + "/", "/" + dirName, "/" + dirName + "/":
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	// Anchored: a folder of that name lower in the tree is the user's.
	line := "/" + dirName + "/\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		line = "\n" + line
	}
	_, err = io.WriteString(f, line)
	return err
}
