//go:build !windows

package shell

import "os/exec"

// hideWindow does nothing outside Windows, where programs open no window.
func hideWindow(*exec.Cmd) {}
