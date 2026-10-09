package term

import (
	"context"
	"io"
	"os"
	"os/exec"
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

// withTerm returns the pane's environment. An inherited environment always
// gets paneTerm, because the outer TERM (for example tmux-256color when Snow
// runs inside tmux) describes a different terminal. A TERM set explicitly in
// spec.Env is kept. Windows programs ignore TERM; WSL and Unix programs need it.
func withTerm(env []string) []string {
	if env == nil {
		return append(dropTerm(os.Environ()), paneTerm)
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "TERM=") {
			return env
		}
	}
	return append(env[:len(env):len(env)], paneTerm)
}

func dropTerm(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "TERM=") {
			out = append(out, kv)
		}
	}
	return out
}

func command(spec Spec) *exec.Cmd {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = withTerm(spec.Env)
	return cmd
}
