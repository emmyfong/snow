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

// withTerm returns the pane's environment: this process's environment with
// TERM set to paneTerm, then spec's additions, which win on conflict. The
// outer TERM (for example tmux-256color when Snow runs inside tmux) describes
// a different terminal than the vt emulator. Windows programs ignore TERM;
// WSL and Unix programs need it.
func withTerm(extra []string) []string {
	env := append(dropTerm(os.Environ()), paneTerm)
	return append(env, extra...)
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
