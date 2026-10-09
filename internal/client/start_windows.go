package client

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// detached starts the server without a console, so closing the terminal
// window does not end it, and in its own process group, so Ctrl+C in the
// window does not reach it.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
