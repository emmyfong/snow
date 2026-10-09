package term

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/xpty"
)

// PTY is a pseudo-terminal with one program running on it.
type PTY interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	// Wait blocks until the program exits. Canceling ctx kills the program.
	Wait(ctx context.Context) error
}

// xptyProcess adapts xpty to PTY. xpty already hides Unix PTYs versus
// Windows ConPTY, so one implementation serves every OS.
type xptyProcess struct {
	xpty.Pty
	cmd *exec.Cmd
}

func startPTY(spec Spec, cols, rows int) (PTY, error) {
	p, err := xpty.NewPty(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = withTerm(spec.Env)
	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	return &xptyProcess{Pty: p, cmd: cmd}, nil
}

// withTerm sets TERM unless the caller did. Windows programs ignore it; WSL
// and Unix programs need it to pick their escape sequences.
func withTerm(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "TERM=") {
			return env
		}
	}
	return append(env[:len(env):len(env)], "TERM=xterm-256color")
}

func (x *xptyProcess) Resize(cols, rows int) error {
	if err := x.Pty.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	return nil
}

// Wait uses xpty.WaitProcess because exec.Cmd.Wait never returns for ConPTY
// processes on Windows.
func (x *xptyProcess) Wait(ctx context.Context) error {
	if err := xpty.WaitProcess(ctx, x.cmd); err != nil {
		return fmt.Errorf("wait for %s: %w", x.cmd.Path, err)
	}
	return nil
}
