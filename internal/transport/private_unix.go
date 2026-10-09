//go:build !windows

package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DefaultPath is $XDG_RUNTIME_DIR/snow/default.sock, or ~/.snow/run/default.sock
// when XDG_RUNTIME_DIR is unset.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "snow", "default.sock"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home folder: %w", err)
	}
	return filepath.Join(home, ".snow", "run", "default.sock"), nil
}

// makePrivateDir creates dir with mode 0700 and refuses a folder that another
// user owns or that is a symbolic link.
func makePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create socket folder: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("check socket folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("socket folder %s is not a plain folder", dir)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("socket folder %s belongs to another user", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("restrict socket folder: %w", err)
	}
	return nil
}

func restrictSocket(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("restrict socket: %w", err)
	}
	return nil
}
