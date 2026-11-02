package shell

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// HideWindow keeps a console program from opening a window: the server
// that starts it may have no visible console.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
}
