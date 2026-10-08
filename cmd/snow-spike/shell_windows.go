package main

import "os/exec"

func defaultShell() string {
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		return "pwsh.exe"
	}
	return "cmd.exe"
}
