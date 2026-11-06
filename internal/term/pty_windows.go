package term

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/conpty"
	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/windows"
)

// windowsPTY runs a program under ConPTY through xpty.
type windowsPTY struct {
	*xpty.ConPty
	cmd *exec.Cmd
}

func startPTY(spec Spec, cols, rows int) (PTY, error) {
	// Without INHERIT_CURSOR, ConPTY paints its whole empty screen over what
	// the pane already shows, such as a preamble. With it, ConPTY asks the
	// emulator for the cursor position and draws from there; the emulator
	// answers through drain. xpty cannot pass flags: its options take
	// Options by value.
	c, err := conpty.New(cols, rows, windows.PSEUDOCONSOLE_INHERIT_CURSOR)
	if err != nil {
		return nil, fmt.Errorf("open conpty: %w", err)
	}
	p := &xpty.ConPty{ConPty: c}
	cmd := command(spec)
	if spec.CmdLine != "" {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: spec.CmdLine}
	}
	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	return &windowsPTY{ConPty: p, cmd: cmd}, nil
}

// Close kills the program, wakes a read blocked on the output pipe, then
// closes the pseudo console. Closing a pipe handle does not wake a
// synchronous ReadFile on it: that read returns only when the console host
// exits, which can lag behind the kill, or never come.
func (w *windowsPTY) Close() error {
	if w.cmd.Process != nil {
		_ = w.cmd.Process.Kill() // fails only when it already exited
	}
	cancelRead(w.OutPipeReadFd())
	if err := w.ConPty.Close(); err != nil {
		return fmt.Errorf("close conpty: %w", err)
	}
	return nil
}

// cancelRead cancels any read pending on the handle h, from any thread.
func cancelRead(h uintptr) {
	_ = windows.CancelIoEx(windows.Handle(h), nil) // fails when no read is pending
}

func (w *windowsPTY) Resize(cols, rows int) error {
	if err := w.ConPty.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize conpty: %w", err)
	}
	return nil
}

// Wait uses xpty.WaitProcess because exec.Cmd.Wait never returns for ConPTY
// processes.
func (w *windowsPTY) Wait(ctx context.Context) error {
	return exitErr(xpty.WaitProcess(ctx, w.cmd))
}
