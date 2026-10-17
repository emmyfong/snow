package transport

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPath is %LOCALAPPDATA%\snow\run\default.sock.
func DefaultPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find local app data folder: %w", err)
	}
	return filepath.Join(cache, "snow", "run", "default.sock"), nil
}

// restrictSocket does nothing on Windows: the socket inherits the folder ACL.
func restrictSocket(string) error { return nil }
