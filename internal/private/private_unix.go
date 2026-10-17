//go:build !windows

// Package private makes folders only the current user can read: the socket
// folder and the saved sessions, which hold terminal output.
package private

import (
	"fmt"
	"os"
	"syscall"
)

// Dir creates dir, private to the current user, with mode 0700. It tightens
// an existing folder's mode and refuses a folder that another
// user owns or that is a symbolic link.
func Dir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("check folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("folder %s is not a plain folder", dir)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("folder %s belongs to another user", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("restrict folder: %w", err)
	}
	return nil
}
