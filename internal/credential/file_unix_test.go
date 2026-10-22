//go:build !windows

package credential

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileMode: the credentials file is mode 0600 in a 0700 folder.
func TestFileMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".snow")
	writeKeys(t, dir, map[string]string{"anthropic": secret})
	for path, want := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, fileName): 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", filepath.Base(path), got, want)
		}
	}
}
