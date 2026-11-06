package clauderuntime

import (
	"path/filepath"
	"strings"
)

// samePath reports whether two native folders are the same. Windows paths
// ignore case, and Claude may report its folder in another case.
func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
