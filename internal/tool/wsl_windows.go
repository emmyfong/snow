//go:build windows

package tool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// wslResolveTimeout bounds one resolve call. wsl.exe can hang while WSL
// starts or shuts down. A cold distribution start needs a few seconds.
const wslResolveTimeout = 10 * time.Second

// resolveWSLPaths returns the canonical paths of linuxPaths inside distro,
// in one wsl.exe call.
func resolveWSLPaths(ctx context.Context, distro string, linuxPaths ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, wslResolveTimeout)
	defer cancel()
	args := append([]string{"-d", distro, "-e", "sh", "-c", wslResolveScript, "sh"}, linuxPaths...)
	cmd := exec.CommandContext(ctx, "wsl.exe", args...)
	shell.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		names := strings.Join(linuxPaths, ", ")
		if ctx.Err() != nil {
			return nil, fmt.Errorf("resolve %s in WSL distribution %s within %s: %w", names, distro, wslResolveTimeout, ctx.Err())
		}
		return nil, fmt.Errorf("resolve %s in WSL distribution %s: %w%s", names, distro, err, wslErrorDetail(err))
	}
	fields := bytes.Split(bytes.TrimRight(out, "\r\n"), []byte{0})
	if len(fields) != len(linuxPaths)+1 || len(fields[len(fields)-1]) != 0 {
		return nil, fmt.Errorf("resolve %s in WSL distribution %s: got %d paths, want %d", strings.Join(linuxPaths, ", "), distro, len(fields)-1, len(linuxPaths))
	}
	resolved := make([]string, len(linuxPaths))
	for i, field := range fields[:len(linuxPaths)] {
		if len(field) == 0 {
			return nil, fmt.Errorf("resolve %s in WSL distribution %s: empty path", strings.Join(linuxPaths, ", "), distro)
		}
		resolved[i] = path.Clean(string(field))
	}
	return resolved, nil
}

// wslErrorDetail returns a short message from what wsl.exe wrote. wsl.exe
// writes its own errors in UTF-16LE; a program inside the distribution
// writes UTF-8.
func wslErrorDetail(err error) string {
	var ee *exec.ExitError
	if !errors.As(err, &ee) || len(ee.Stderr) == 0 {
		return ""
	}
	msg := string(ee.Stderr)
	if bytes.IndexByte(ee.Stderr, 0) >= 0 {
		msg = shell.DecodeUTF16LE(ee.Stderr)
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return ": " + msg
}

// wslOSPath returns the Windows path of a Linux path in distro.
func wslOSPath(distro, linux string) string { return shell.WSLToUNC(distro, linux) }
