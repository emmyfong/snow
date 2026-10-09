package client

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// detached starts the server with its own hidden console, so closing the
// terminal window does not end it, and in its own process group, so Ctrl+C
// in the window does not reach it. DETACHED_PROCESS would give it no console
// at all; then every console program it runs, such as wsl.exe for shell
// detection, opens a new visible window.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
