//go:build !windows

package transport

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive, non-blocking lock on f. The OS releases it
// when the process exits, even after a crash.
func lockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) //nolint:gosec // file descriptors fit in int
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrInUse
	}
	if err != nil {
		return fmt.Errorf("lock %s: %w", f.Name(), err)
	}
	return nil
}
