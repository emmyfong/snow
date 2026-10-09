//go:build !windows

package term

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// unixPTY uses creack/pty directly instead of xpty. xpty keeps the child's
// end of the PTY open in this process, so reads never end after the child
// exits, and it does not give the child a controlling terminal.
type unixPTY struct {
	*os.File // the parent's end of the PTY
	cmd      *exec.Cmd
}

func startPTY(spec Spec, cols, rows int) (PTY, error) {
	cmd := command(spec)
	// StartWithSize makes the PTY the child's controlling terminal in a new
	// session (process group) and closes the child's end in this process.
	f, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	return &unixPTY{File: f, cmd: cmd}, nil
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)} //nolint:gosec // terminal sizes fit in uint16
}

func (u *unixPTY) Resize(cols, rows int) error {
	if err := pty.Setsize(u.File, winsize(cols, rows)); err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	return nil
}

// Wait kills the child's whole process group when ctx is canceled, so a
// program the shell started (for example `sleep` under `sh -c`) dies too.
func (u *unixPTY) Wait(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- u.cmd.Wait() }()
	select {
	case err := <-done:
		return exitErr(err)
	case <-ctx.Done():
		_ = syscall.Kill(-u.cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return ctx.Err()
	}
}

// exitErr treats a program's non-zero exit status as a normal exit.
func exitErr(err error) error {
	var exit *exec.ExitError
	if err == nil || errors.As(err, &exit) {
		return nil
	}
	return fmt.Errorf("wait: %w", err)
}
