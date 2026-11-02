package notify

import (
	"context"
	"os/exec"
)

// command returns the Windows toast helper.
func command(ctx context.Context, n Notice) *exec.Cmd {
	return toastCommand(ctx, n, "powershell.exe")
}
