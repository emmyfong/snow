package files

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// Walk calls fn for every file and folder under root, an OS path, in
// lexical order, with its path from root in slash form. It skips .git
// folders and whatever .gitignore files and .git/info/exclude ignore, as
// git does. fn returns fs.SkipDir to skip a folder's contents, and
// fs.SkipAll to end the walk.
func Walk(ctx context.Context, root string, fn func(rel string, d fs.DirEntry) error) error {
	patterns := readIgnore(filepath.Join(root, ".git", "info", "exclude"), nil)
	err := walkDir(ctx, root, nil, patterns, fn)
	if errors.Is(err, fs.SkipAll) {
		return nil
	}
	return err
}

func walkDir(ctx context.Context, dir string, parts []string, patterns []gitignore.Pattern, fn func(string, fs.DirEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	patterns = append(patterns[:len(patterns):len(patterns)], readIgnore(filepath.Join(dir, ".gitignore"), parts)...)
	m := gitignore.NewMatcher(patterns)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == ".git" && e.IsDir() {
			continue
		}
		p := append(parts[:len(parts):len(parts)], name)
		if m.Match(p, e.IsDir()) {
			continue
		}
		if err := fn(strings.Join(p, "/"), e); err != nil {
			if errors.Is(err, fs.SkipDir) {
				continue
			}
			return err
		}
		if e.IsDir() {
			if err := walkDir(ctx, filepath.Join(dir, name), p, patterns, fn); err != nil {
				return err
			}
		}
	}
	return nil
}
