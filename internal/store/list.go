package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Info describes a saved session for a list, without its panes' lines.
type Info struct {
	Name    string
	Saved   time.Time
	Windows int
}

// List reads every snapshot in the folder, sorted by session name. A file
// that cannot be read back is returned as an error, a *DecodeError when its
// content is bad, so the caller can report it; it is not skipped silently.
// Temp files that an interrupted write left are not snapshots.
func (s *Store) List() ([]Info, []error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, []error{fmt.Errorf("list sessions: %w", err)}
	}
	var (
		infos []Info
		errs  []error
	)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("list sessions: %w", err))
			continue
		}
		snap, err := decode(b)
		if de, ok := errors.AsType[*DecodeError](err); ok {
			de.Path = path
			errs = append(errs, de)
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("list sessions: %s: %w", path, err))
			continue
		}
		infos = append(infos, Info{Name: snap.Name, Saved: snap.Saved, Windows: len(snap.Windows)})
	}
	slices.SortFunc(infos, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })
	return infos, errs
}
