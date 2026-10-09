// Package transport connects Snow's client and server over a Unix domain
// socket that only the current user can reach. Windows 10 1803 and later
// support Unix domain sockets natively, so every OS uses the same code.
package transport

import (
	"errors"
	"fmt"
	"io/fs"
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
	ErrPathInUse   = errors.New("transport: socket path holds something else")
)

const probeTimeout = time.Second

// Listen creates the socket at path for a server. It makes the socket's
// folder private to the current user first.
//
// A lock file next to the socket makes start-up exclusive: only one process
// can check, replace, and listen at a time, and the winner holds the lock
// until its listener closes. Others get ErrInUse. A socket left by a crashed
// server is replaced; anything else at path is left alone and reported.
func Listen(path string) (net.Listener, error) {
	if len(path) > maxPathLen {
		return nil, fmt.Errorf("%w: %d bytes: %s", ErrPathTooLong, len(path), path)
	}
	if err := makePrivateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := lockFile(lock); err != nil {
		_ = lock.Close()
		if errors.Is(err, ErrInUse) {
			return nil, fmt.Errorf("%w: %s", ErrInUse, path)
		}
		return nil, err
	}
	l, err := listenLocked(path)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	return &lockedListener{Listener: l, lock: lock}, nil
}

// listenLocked replaces a stale socket and listens. The caller holds the lock.
func listenLocked(path string) (net.Listener, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("check socket path: %w", err)
	case info.Mode()&os.ModeSocket == 0:
		return nil, fmt.Errorf("%w: %s is not a socket", ErrPathInUse, path)
	default:
		c, err := net.DialTimeout("unix", path, probeTimeout)
		if err == nil {
			// A server that predates the lock file is still running.
			_ = c.Close()
			return nil, fmt.Errorf("%w: %s", ErrInUse, path)
		}
		if !isRefused(err) {
			return nil, fmt.Errorf("probe existing socket: %w", err)
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

// lockedListener releases the start-up lock when the listener closes.
type lockedListener struct {
	net.Listener
	lock *os.File
}

func (l *lockedListener) Close() error {
	return errors.Join(l.Listener.Close(), l.lock.Close())
}

// Dial connects a client to the server at path. It returns an error matching
// ErrNoServer only when no server is there: no socket file, or a socket file
// with nobody listening. Other failures, such as a timeout against a busy
// server, are returned as they are, so a client does not start a second
// server next to a live one.
func Dial(path string) (net.Conn, error) {
	c, err := net.DialTimeout("unix", path, probeTimeout)
	if err != nil {
		if isRefused(err) || isAbsent(err) {
			return nil, fmt.Errorf("%w: %w", ErrNoServer, err)
		}
		return nil, fmt.Errorf("connect to %s: %w", path, err)
	}
	return c, nil
}
