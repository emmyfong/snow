//go:build !windows

package client

import "syscall"

// detached puts the server in a new session with no controlling terminal, so
// closing the terminal window does not send it a hang-up signal.
func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
