package client

import (
	"fmt"
	"os"
	"os/exec"
)

// StartServer launches exe with args as a detached background process: it
// keeps running after this process and its terminal window close.
func StartServer(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = detached()
	// The server outlives this terminal. Starting it in the home folder
	// keeps it from holding the client's folder open (Windows cannot
	// delete a folder in use); sessions get their folder from Attach.
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", exe, err)
	}
	// The server outlives this process; nobody waits for it.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release server process: %w", err)
	}
	return nil
}
