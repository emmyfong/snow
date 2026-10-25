//go:build !windows

package tool

import "path/filepath"

// evalLinks returns p with every link resolved.
func evalLinks(p string) (string, error) { return filepath.EvalSymlinks(p) }
