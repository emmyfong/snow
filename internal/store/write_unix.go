//go:build !windows

package store

import (
	"errors"
	"os"
)

// rename replaces to with from. POSIX rename is atomic and replaces an
// existing file even while another process has it open.
func rename(from, to string) error { return os.Rename(from, to) }

// remove deletes path. POSIX removes a file even while it is open.
func remove(path string) error { return os.Remove(path) }

// syncDir flushes the folder entry, so the rename itself survives a power
// cut and not only the file data.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}
