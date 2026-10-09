// Package transport connects Snow's client and server over a Unix domain
// socket that only the current user can reach. Windows 10 1803 and later
// support Unix domain sockets natively, so every OS uses the same code.
package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// maxPathLen stays under the OS limit on socket paths (104 bytes on macOS,
// 108 on Linux and Windows, both including a terminating zero byte).
const maxPathLen = 100

// Transport errors. Callers match them with errors.Is.
var (
	ErrInUse       = errors.New("transport: a server is already listening")
	ErrNoServer    = errors.New("transport: no server is listening")
	ErrPathTooLong = errors.New("transport: socket path is too long")
)

const probeTimeout = time.Second

// Listen creates the socket at path for a server. It makes the socket's
// folder private to the current user first. A socket file left by a crashed
// server is replaced; a live server makes Listen return ErrInUse.
func Listen(path string) (net.Listener, error) {
	if len(path) > maxPathLen {
		return nil, fmt.Errorf("%w: %d bytes: %s", ErrPathTooLong, len(path), path)
	}
	if err := makePrivateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		if c, err := net.DialTimeout("unix", path, probeTimeout); err == nil {
			_ = c.Close()
			return nil, fmt.Errorf("%w: %s", ErrInUse, path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	if err := restrictSocket(path); err != nil {
		_ = l.Close()
		return nil, err
	}
	return l, nil
}

// Dial connects a client to the server at path. It returns an error matching
// ErrNoServer when nothing is listening.
func Dial(path string) (net.Conn, error) {
	c, err := net.DialTimeout("unix", path, probeTimeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoServer, err)
	}
	return c, nil
}
