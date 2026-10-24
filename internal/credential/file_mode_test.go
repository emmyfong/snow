package credential

import (
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/private/privatetest"
)

// TestFileMode: the credentials file and its folder are private to the
// current user.
func TestFileMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".snow")
	writeKeys(t, dir, map[string]string{"anthropic": secret})
	privatetest.OwnerOnly(t, dir, 0o700)
	privatetest.OwnerOnly(t, filepath.Join(dir, fileName), 0o600)
}
