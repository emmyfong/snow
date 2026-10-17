//go:build !windows

package main

// stopHint is the command that stops a server too old to restart itself.
const stopHint = "pkill -f 'snow server'"
