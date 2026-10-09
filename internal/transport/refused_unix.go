//go:build !windows

package transport

import (
	"errors"
	"io/fs"
	"syscall"
)

// isRefused reports whether a dial found a socket file with nobody listening.
func isRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

// isAbsent reports whether a dial found no socket file.
func isAbsent(err error) bool { return errors.Is(err, fs.ErrNotExist) }
