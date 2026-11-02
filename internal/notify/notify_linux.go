package notify

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// command returns the Linux notification helper. In WSL it reaches the
// Windows desktop through interop instead.
func command(ctx context.Context, n Notice) *exec.Cmd {
	if wsl() {
		return toastCommand(ctx, n, "powershell.exe")
	}
	title, body := message(n)
	return exec.CommandContext(ctx, "notify-send", "-a", "Snow", title, body)
}

// wsl reports whether Snow runs in WSL, where notify-send has no desktop to
// reach.
func wsl() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	release, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(release)), "microsoft")
}
