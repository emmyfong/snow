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
	"golang.org/x/sys/unix"
)

// unixPTY opens the PTY with creack/pty and starts the child itself. xpty
// keeps the child's end open in this process, so reads never end after the
// child exits, and gives no controlling terminal; creack's StartWithSize
// resizes through f.Fd(), which makes the parent's end blocking.
type unixPTY struct {
	*os.File // the parent's end of the PTY
	cmd      *exec.Cmd
}

func startPTY(spec Spec, cols, rows int) (PTY, error) {
	blocking, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	ptm, err := pollable(blocking)
	if err != nil {
		_ = tty.Close()
		return nil, err
	}
	if err := setWinsize(tty, cols, rows); err != nil {
		_ = ptm.Close()
		_ = tty.Close()
		return nil, err
	}
	cmd := command(spec)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	// A new session with the PTY as controlling terminal (Ctty is the
	// child's descriptor 0), so job control and Ctrl+C work.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	err = cmd.Start()
	// The child has its own copy; keeping ours would hide the child's exit.
	_ = tty.Close()
	if err != nil {
		_ = ptm.Close()
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	return &unixPTY{File: ptm, cmd: cmd}, nil
}

// pollable returns a non-blocking copy of f and closes f. creack/pty's Open
// calls f.Fd(), which leaves the descriptor blocking; a blocking Write stuck
// on a program that stopped reading cannot be interrupted by Close. os.NewFile
// returns a file Go's poller manages when the descriptor is non-blocking.
func pollable(f *os.File) (*os.File, error) {
	defer func() { _ = f.Close() }()
	fd, err := unix.Dup(int(f.Fd())) //nolint:gosec // file descriptors fit in int
	if err != nil {
		return nil, fmt.Errorf("dup pty: %w", err)
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("set pty non-blocking: %w", err)
	}
	return os.NewFile(uintptr(fd), f.Name()), nil
}

// setWinsize sets the terminal size through SyscallConn. It must not use
// f.Fd(): that switches the file to blocking mode for good, and then Close
// can no longer interrupt a Write stuck on a program that stopped reading.
func setWinsize(f *os.File, cols, rows int) error {
	sc, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	ws := &unix.Winsize{Col: uint16(cols), Row: uint16(rows)} //nolint:gosec // sizes are bounded by the server
	var ioctlErr error
	if err := sc.Control(func(fd uintptr) {
		ioctlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, ws) //nolint:gosec // file descriptors fit in int
	}); err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	if ioctlErr != nil {
		return fmt.Errorf("resize pty: %w", ioctlErr)
	}
	return nil
}

func (u *unixPTY) Resize(cols, rows int) error { return setWinsize(u.File, cols, rows) }

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
