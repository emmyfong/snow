//go:build !windows

package shell

import "os/exec"

// HideWindow does nothing outside Windows, where programs open no window.
func HideWindow(*exec.Cmd) {}
