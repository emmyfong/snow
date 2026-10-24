package private

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// TempPath is where WriteAtomic writes before the rename. A crash can leave
// it behind; the next write replaces it.
func TempPath(path string) string { return path + ".tmp" }

// WriteAtomic replaces the file at path with what write produces. Readers
// see the old file or the whole new one, never a mix: the data goes to a
// temp file in the same folder, is flushed to disk, and is then renamed over
// path in one step. A crash before the rename leaves the old file intact.
func WriteAtomic(path string, write func(io.Writer) error) (err error) {
	tmp := TempPath(path)
	// Remove a stale temp file so O_EXCL creates a fresh one with mode 0600;
	// opening an existing file keeps its old mode.
	if err := Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove stale temp file: %w", err)
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, removeTemp(tmp))
		}
	}()
	if err := write(f); err != nil {
		return errors.Join(fmt.Errorf("write temp file: %w", err), f.Close())
	}
	// Sync before the rename, or a power cut can leave the new name pointing
	// at a file whose data never reached the disk.
	if err := f.Sync(); err != nil {
		return errors.Join(fmt.Errorf("flush temp file: %w", err), f.Close())
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	if err := SyncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("flush folder: %w", err)
	}
	return nil
}

func removeTemp(tmp string) error {
	if err := Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove temp file: %w", err)
	}
	return nil
}

// Bytes returns a write function for WriteAtomic that writes b.
func Bytes(b []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	}
}
