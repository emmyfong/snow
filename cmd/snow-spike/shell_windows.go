package main

import "os/exec"

func defaultShell() string {
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "cmd.exe"
}
