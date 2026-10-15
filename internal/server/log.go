package server

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// logName is the server's log file, next to its socket.
const logName = "server.log"

// maxLogSize is the size at which OpenLog starts a new log. The previous
// log is kept as server.log.old.
const maxLogSize = 1 << 20

// OpenLog opens the server log in dir for appending. A log over maxLogSize
// is moved to server.log.old first, so a long-running server cannot fill
// the disk. Close the returned Closer when the server stops.
func OpenLog(dir string) (*slog.Logger, io.Closer, error) {
	path := filepath.Join(dir, logName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogSize {
		// Windows cannot rename over an existing file.
		_ = os.Remove(path + ".old")
		if err := os.Rename(path, path+".old"); err != nil {
			return nil, nil, fmt.Errorf("rotate log: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	return slog.New(slog.NewTextHandler(f, nil)), f, nil
}
