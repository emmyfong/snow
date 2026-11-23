package git

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrEmptyMessage means a commit was asked for with no message.
// ErrConflictMarkers means a file still holds conflict markers.
var (
	ErrEmptyMessage    = errors.New("the commit message is empty")
	ErrConflictMarkers = errors.New("conflict markers remain")
)

// Stage stages the changes to paths, which are from the top of the
// repository. A deleted file's deletion is staged too.
func (r Repo) Stage(ctx context.Context, paths ...string) error {
	if files, err := r.ConflictMarkers(ctx, false, paths...); err != nil {
		return err
	} else if len(files) != 0 {
		return conflictMarkersError(files)
	}
	return r.onPaths(ctx, []string{"add", "--all", "--"}, paths)
}

// StageAll stages every change in the repository, also those the pane
// does not list, with one short command line.
func (r Repo) StageAll(ctx context.Context) error {
	if files, err := r.ConflictMarkers(ctx, false); err != nil {
		return err
	} else if len(files) != 0 {
		return conflictMarkersError(files)
	}
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
	if files, err := r.ConflictMarkers(ctx, true); err != nil {
		return "", err
	} else if len(files) != 0 {
		return "", conflictMarkersError(files)
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

// ConflictMarkers returns the files among paths that hold conflict markers.
// An empty paths list checks every file. When cached is true, it checks the
// index and checks working files only for unresolved paths.
func (r Repo) ConflictMarkers(ctx context.Context, cached bool, paths ...string) ([]string, error) {
	if !cached {
		return r.workingConflictMarkers(ctx, r.Path, paths, false)
	}

	out, err := r.Run(ctx, r.Path, "diff", "--cached", "--check")
	if err != nil {
		var gitErr *Error
		if !errors.As(err, &gitErr) || gitErr.Code != 2 {
			return nil, r.Classify(err)
		}
	}
	found := markerFiles(out)
	if len(found) != 0 {
		return found, nil
	}

	// git diff --cached --check omits unmerged index entries. Inspect only
	// those files in the work tree so a user still gets the marker warning.
	root, err := r.Run(ctx, r.Path, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, r.Classify(err)
	}
	root = strings.TrimSpace(root)
	unmerged, err := r.unmergedPaths(ctx, root)
	if err != nil {
		return nil, err
	}
	unmerged = selectPaths(unmerged, paths)
	if len(unmerged) == 0 {
		return nil, nil
	}
	return r.workingConflictMarkers(ctx, root, unmerged, true)
}

func (r Repo) workingConflictMarkers(ctx context.Context, dir string, paths []string, noIndex bool) ([]string, error) {
	args := []string{"grep", "--basic-regexp"}
	if noIndex {
		args = append(args, "--no-index")
	} else {
		args = append(args, "--untracked")
	}
	args = append(args, "-l", "-z")
	for _, pattern := range []string{"^<<<<<<< ", "^=======$", "^>>>>>>> "} {
		args = append(args, "-e", pattern)
	}
	args = append(args, "--")
	if len(paths) == 0 {
		if noIndex {
			args = append(args, ".")
		} else {
			args = append(args, ":(top)")
		}
	} else {
		for _, path := range paths {
			if noIndex {
				args = append(args, path)
			} else {
				args = append(args, pathspec(path))
			}
		}
	}
	out, err := r.Run(ctx, dir, args...)
	if err != nil {
		if Declined(err) {
			return nil, nil
		}
		if !noIndex && errors.Is(r.Classify(err), ErrNotRepo) {
			return r.workingConflictMarkers(ctx, dir, paths, true)
		}
		return nil, r.Classify(err)
	}
	files := strings.Split(out, "\x00")
	if len(files) > 0 && files[len(files)-1] == "" {
		files = files[:len(files)-1]
	}
	sort.Strings(files)
	return files, nil
}

func (r Repo) unmergedPaths(ctx context.Context, root string) ([]string, error) {
	out, err := r.Run(ctx, root, "ls-files", "--unmerged", "-z")
	if err != nil {
		return nil, r.Classify(err)
	}
	seen := make(map[string]struct{})
	for _, entry := range strings.Split(out, "\x00") {
		_, path, ok := strings.Cut(entry, "\t")
		if ok {
			seen[path] = struct{}{}
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func selectPaths(files, paths []string) []string {
	if len(paths) == 0 {
		return files
	}
	selected := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		selected[path] = struct{}{}
	}
	result := make([]string, 0, len(files))
	for _, file := range files {
		if _, ok := selected[file]; ok {
			result = append(result, file)
		}
	}
	return result
}

func markerFiles(output string) []string {
	const marker = ": leftover conflict marker"
	found := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		markerAt := strings.LastIndex(line, marker)
		if markerAt < 0 {
			continue
		}
		lineAt := strings.LastIndex(line[:markerAt], ":")
		if lineAt < 0 {
			continue
		}
		found[line[:lineAt]] = struct{}{}
	}
	files := make([]string, 0, len(found))
	for file := range found {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func conflictMarkersError(files []string) error {
	return fmt.Errorf("%w in %s. Remove them, then stage again", ErrConflictMarkers, strings.Join(files, ", "))
}
