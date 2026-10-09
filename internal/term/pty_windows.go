package term

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/charmbracelet/x/xpty"
)

// windowsPTY runs a program under ConPTY through xpty.
type windowsPTY struct {
	*xpty.ConPty
	cmd *exec.Cmd
}

func startPTY(spec Spec, cols, rows int) (PTY, error) {
	p, err := xpty.NewConPty(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("open conpty: %w", err)
	}
	cmd := command(spec)
	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("start %s: %w", spec.Path, err)
	}
	return &windowsPTY{ConPty: p, cmd: cmd}, nil
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
	if err := xpty.WaitProcess(ctx, w.cmd); err != nil {
		return fmt.Errorf("wait for %s: %w", w.cmd.Path, err)
	}
	return nil
}
