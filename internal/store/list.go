package store

import (
	"errors"
	"fmt"
	"github.com/emmyfong/snow/internal/private"
	"io/fs"
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
		snap, err := read(filepath.Join(s.dir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("list sessions: %w", err))
			continue
		}
		infos = append(infos, Info{Name: snap.Name, Saved: snap.Saved, Windows: len(snap.Windows)})
	}
	slices.SortFunc(infos, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })
	return infos, errs
}

// MoveBroken moves a snapshot file that cannot be read back to the broken
// folder beside the snapshots, named after it and stamped with at, and
// returns its new path. The file is kept for the user, never repaired.
func (s *Store) MoveBroken(path string, at time.Time) (string, error) {
	dir := filepath.Join(s.dir, "broken")
	if err := private.Dir(dir); err != nil {
		return "", fmt.Errorf("move broken snapshot: %w", err)
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".json") + "-" + at.UTC().Format("20060102T150405")
	moved := filepath.Join(dir, stem+".json")
	// rename replaces a file of the same name; two broken files from one
	// second must both stay.
	for n := 2; ; n++ {
		if _, err := os.Lstat(moved); errors.Is(err, fs.ErrNotExist) {
			break
		}
		moved = filepath.Join(dir, fmt.Sprintf("%s-%d.json", stem, n))
	}
	if err := private.Rename(path, moved); err != nil {
		return "", fmt.Errorf("move broken snapshot: %w", err)
	}
	return moved, nil
}
