// Package files reads one folder at a time for the files pane, in the
// order git would list them: .gitignore leaves files out, and each file's
// status mark comes from git.
package files

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/emmyfong/snow/internal/shell"
)

// Entry is one file or folder in a listing.
type Entry struct {
	Name string
	Path string // from the listing's root, with forward slashes
	Dir  bool
}

// MaxEntries bounds one listing, as api.MaxFilesEntries does.
const MaxEntries = 2000

// List returns the direct children of the folder at rel, a path from root
// with forward slashes, sorted with folders first and then by name. It
// skips .git folders and whatever .gitignore files and .git/info/exclude
// ignore. root is the path the OS Snow runs on uses. omitted counts the
// entries past MaxEntries.
func List(ctx context.Context, root, rel string) (entries []Entry, omitted int, err error) {
	dir, err := join(root, rel)
	if err != nil {
		return nil, 0, err
	}
	pat, err := patterns(root, rel)
	if err != nil {
		return nil, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	m := gitignore.NewMatcher(pat)
	parts := split(rel)
	entries = make([]Entry, 0, len(dirEntries))
	for _, e := range dirEntries {
		name := e.Name()
		if name == ".git" && e.IsDir() {
			continue
		}
		parts := append(append([]string(nil), parts...), name)
		if m.Match(parts, e.IsDir()) {
			continue
		}
		if len(entries) == MaxEntries {
			omitted++
			continue
		}
		entries = append(entries, Entry{Name: name, Path: strings.Join(parts, "/"), Dir: e.IsDir()})
	}
	slices.SortStableFunc(entries, func(a, b Entry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return entries, omitted, nil
}

// join returns the OS path of rel below root.
func join(root, rel string) (string, error) {
	if rel == "" {
		return root, nil
	}
	for _, part := range split(rel) {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path %q is not below the folder", rel)
		}
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

// split cuts a relative path into its parts.
func split(rel string) []string {
	if rel == "" {
		return nil
	}
	return strings.Split(rel, "/")
}

// patterns reads .git/info/exclude and every .gitignore from root down to
// rel, each with its folder as the domain git matches against.
func patterns(root, rel string) ([]gitignore.Pattern, error) {
	out := readIgnore(filepath.Join(root, ".git", "info", "exclude"), nil)
	parts := split(rel)
	for i := 0; i <= len(parts); i++ {
		domain := parts[:i]
		file := filepath.Join(root, filepath.FromSlash(path.Join(domain...)), ".gitignore")
		out = append(out, readIgnore(file, domain)...)
	}
	return out, nil
}

// readIgnore parses one ignore file. A missing or unreadable file has no
// patterns, as git treats it.
func readIgnore(file string, domain []string) []gitignore.Pattern {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var ps []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ps = append(ps, gitignore.ParsePattern(line, domain))
	}
	return ps
}

// OSPath returns the path the OS Snow runs on uses to reach folder, which
// belongs to distro when distro is not empty.
func OSPath(folder, distro string) string {
	if distro == "" {
		return folder
	}
	return shell.WSLToUNC(distro, folder)
}
