package tool

import (
	"context"
	"fmt"
)

// resolveWSLPath returns the canonical path of linux inside distro.
func resolveWSLPath(ctx context.Context, distro, linux string) (string, error) {
	paths, err := resolveWSLPaths(ctx, distro, linux)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// RealPath returns where Snow opens p. A WSL folder resolves the links inside
// its distro first: Windows cannot follow a WSL link through
// \\wsl.localhost. The approval step records the target, so a link that
// changes after approval cannot redirect the open.
func (e Env) RealPath(ctx context.Context, p Path) (string, error) {
	if e.Folder.Distro == "" {
		return p.OS, nil
	}
	if target, ok := e.Approved[p.Logical]; ok {
		if target == "" {
			return "", fmt.Errorf("resolve %s: approval could not resolve it", p.Logical)
		}
		return wslOSPath(e.Folder.Distro, target), nil
	}
	target, err := resolveWSLPath(ctx, e.Folder.Distro, p.Logical)
	if err != nil {
		return "", err
	}
	return wslOSPath(e.Folder.Distro, target), nil
}
