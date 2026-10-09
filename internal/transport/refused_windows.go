package transport

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/windows"
)

// isRefused reports whether a dial found a socket file with nobody listening.
func isRefused(err error) bool { return errors.Is(err, windows.WSAECONNREFUSED) }

// isAbsent reports whether a dial found no socket file. Windows reports a
// missing AF_UNIX path as WSAENETDOWN ("a socket operation encountered a dead
// network"), not as file not found.
func isAbsent(err error) bool {
	return errors.Is(err, windows.WSAENETDOWN) || errors.Is(err, fs.ErrNotExist)
}
