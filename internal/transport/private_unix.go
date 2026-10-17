//go:build !windows

package transport

import (
	"fmt"
	"os"
	"path/filepath"
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

func restrictSocket(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("restrict socket: %w", err)
	}
	return nil
}
