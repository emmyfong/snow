package store

import (
	"errors"
	"fmt"
	"github.com/emmyfong/snow/internal/private"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// DefaultDir is ~/.snow/sessions on every OS.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home folder: %w", err)
	}
	return filepath.Join(home, ".snow", "sessions"), nil
}

// Store reads and writes session snapshots in one folder, one file per
// session. Callers serialize writes to the same session; Saver does.
type Store struct {
	dir string
}

// Open returns a Store for dir and creates the folder, private to the
// current user, if it is missing.
func Open(dir string) (*Store, error) {
	// Snapshots hold terminal output: the folder is the user's alone.
	if err := private.Dir(dir); err != nil {
		return nil, fmt.Errorf("create sessions folder: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(session string) (string, error) {
	name, err := fileName(session)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.dir, name), nil
}

// Save writes the snapshot atomically to the file for snap.Name, stamps the
// saved time, and keeps the last MaxScrollback lines of each pane.
func (s *Store) Save(snap Snapshot) error {
	path, err := s.path(snap.Name)
	if err != nil {
		return err
	}
	snap.Saved = time.Now().UTC()
	b, err := encode(snap)
	if err != nil {
		return err
	}
	if err := private.WriteAtomic(path, private.Bytes(b)); err != nil {
		return fmt.Errorf("save session %q: %w", snap.Name, err)
	}
	return nil
}

// Load reads the snapshot for session. A missing file gives an error that
// matches fs.ErrNotExist; a file that cannot be decoded gives a
// *DecodeError.
func (s *Store) Load(session string) (Snapshot, error) {
	path, err := s.path(session)
	if err != nil {
		return Snapshot{}, err
	}
	snap, err := read(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load session %q: %w", session, err)
	}
	return snap, nil
}

// Delete removes the snapshot for session and any temp file a crash left.
// A session with no file is not an error.
func (s *Store) Delete(session string) error {
	path, err := s.path(session)
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range []string{path, private.TempPath(path)} {
		if err := private.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("delete session %q: %w", session, err))
		}
	}
	return errors.Join(errs...)
}

// Move renames the saved session from to to: it writes the snapshot under
// to, then deletes from, so a failure part way leaves at least one copy.
func (s *Store) Move(from, to string) error {
	snap, err := s.Load(from)
	if err != nil {
		return err
	}
	snap.Name = to
	if err := s.Save(snap); err != nil {
		return err
	}
	return s.Delete(from)
}

// read reads the snapshot at path. A file that is not a regular file, is
// too big, cannot be decoded, or holds another session than its name says
// gives a *DecodeError with the path; a newer format gives ErrNewerVersion.
func read(path string) (Snapshot, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return Snapshot{}, err
	}
	switch {
	case !fi.Mode().IsRegular():
		return Snapshot{}, &DecodeError{Path: path, Err: errors.New("not a regular file")}
	case fi.Size() > maxFileSize:
		return Snapshot{}, &DecodeError{Path: path, Err: fmt.Errorf("%d bytes, more than %d", fi.Size(), maxFileSize)}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	snap, err := decode(b)
	if de, ok := errors.AsType[*DecodeError](err); ok {
		de.Path = path
		return Snapshot{}, de
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("%s: %w", path, err)
	}
	if name, nerr := fileName(snap.Name); nerr != nil || name != filepath.Base(path) {
		return Snapshot{}, &DecodeError{Path: path, Err: fmt.Errorf("holds session %q, not the one its file name says", snap.Name)}
	}
	return snap, nil
}
