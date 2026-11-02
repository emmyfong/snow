package term

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
)

// PTY is a pseudo-terminal with one program running on it.
type PTY interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	// Wait blocks until the program exits. Canceling ctx kills the program.
	Wait(ctx context.Context) error
}

// paneTerm is the terminal type every pane presents: vt emulates xterm.
const paneTerm = "TERM=xterm-256color"

// withTerm returns the pane's environment: this process's environment
// without the variables named in hide and with TERM set to paneTerm, then
// spec's additions, which win on conflict. The outer TERM (for example
// tmux-256color when Snow runs inside tmux) describes a different terminal
// than the vt emulator. Windows programs ignore TERM; WSL and Unix programs
// need it.
func withTerm(extra, hide []string) []string {
	env := append(drop(os.Environ(), append([]string{"TERM"}, hide...)), paneTerm)
	return append(env, extra...)
}

// drop returns env without the variables named in names. Windows names
// ignore case.
func drop(env, names []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(names, func(n string) bool { return sameVar(n, name) }) {
			out = append(out, kv)
		}
	}
	return out
}

// sameVar reports whether two variable names are the same variable.
func sameVar(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func command(spec Spec) *exec.Cmd {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = withTerm(spec.Env, spec.Hide)
	return cmd
}
