package main

import "github.com/emmyfong/snow/internal/transport"

// unsetHint is the shell command that clears the nested-session marker. A
// Windows pane runs cmd or PowerShell, which clear variables differently.
func unsetHint() string {
	return "set " + transport.EnvName + "= (cmd) or Remove-Item Env:" + transport.EnvName + " (PowerShell)"
}
