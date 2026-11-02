package notify

import (
	"context"
	"os/exec"
)

// appleScript shows the two arguments as a notification: title first, body
// second.
const appleScript = `on run argv
display notification (item 2 of argv) with title (item 1 of argv)
end run`

// command returns the macOS notification helper.
func command(ctx context.Context, n Notice) *exec.Cmd {
	title, body := message(n)
	return exec.CommandContext(ctx, "osascript", "-e", appleScript, title, body)
}
