package files

import (
	"context"
	"io/fs"
)

// ListFiles returns the files below root, up to limit, in walk order, and
// whether a file past the limit was left out.
func ListFiles(ctx context.Context, root string, limit int) (paths []string, cut bool, err error) {
	if limit <= 0 {
		return nil, false, nil
	}
	err = Walk(ctx, root, func(rel string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		if len(paths) >= limit {
			cut = true
			return fs.SkipAll
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return paths, cut, nil
}
