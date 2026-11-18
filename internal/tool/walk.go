package tool

import (
	"context"
	"errors"
	"io/fs"

	"github.com/emmyfong/snow/internal/files"
)

// errStop ends a walk early without an error.
var errStop = errors.New("stop walking")

// walk calls fn for every file and folder under root (an OS path), in
// lexical order, with its path relative to root in slash form. It skips
// .git folders and whatever .gitignore files and .git/info/exclude
// ignore, as git does. fn returns errStop to end the walk.
func walk(ctx context.Context, root string, fn func(rel string, d fs.DirEntry) error) error {
	return files.Walk(ctx, root, func(rel string, d fs.DirEntry) error {
		err := fn(rel, d)
		if errors.Is(err, errStop) {
			return fs.SkipAll
		}
		return err
	})
}
