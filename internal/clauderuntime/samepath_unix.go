//go:build !windows

package clauderuntime

import "path/filepath"

// samePath reports whether two native folders are the same.
func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }
