//go:build !windows

package tool

import "context"

// resolveWSLPaths returns linuxPaths unchanged: a WSL folder is only in use
// on Windows.
func resolveWSLPaths(_ context.Context, _ string, linuxPaths ...string) ([]string, error) {
	return linuxPaths, nil
}

// wslOSPath returns linux unchanged: a WSL folder is only in use on Windows.
func wslOSPath(_, linux string) string { return linux }
