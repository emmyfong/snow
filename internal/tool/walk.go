package tool

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// errStop ends a walk early without an error.
var errStop = errors.New("stop walking")

// walk calls fn for every file and folder under root (an OS path), in
// lexical order, with its path relative to root in slash form. It skips
// .git folders and whatever .gitignore files and .git/info/exclude
// ignore, as git does. fn returns errStop to end the walk.
func walk(ctx context.Context, root string, fn func(rel string, d fs.DirEntry) error) error {
	patterns := readIgnore(filepath.Join(root, ".git", "info", "exclude"), nil)
	err := walkDir(ctx, root, nil, patterns, fn)
	if errors.Is(err, errStop) {
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

// readIgnore parses one ignore file; domain is its folder's path parts. A
// missing or unreadable file has no patterns.
func readIgnore(file string, domain []string) []gitignore.Pattern {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var ps []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ps = append(ps, gitignore.ParsePattern(line, domain))
	}
	return ps
}
