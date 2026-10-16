//go:build !windows

package main

import "github.com/emmyfong/snow/internal/transport"

// unsetHint is the shell command that clears the nested-session marker.
func unsetHint() string { return "unset " + transport.EnvName }
