package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// shell is one program running on a PTY, with an emulator holding its screen.
type shell struct {
	pty xpty.Pty
	emu *vt.SafeEmulator
	cmd *exec.Cmd
}

func startShell(name string, args []string, width, height int) (*shell, error) {
	p, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	emu := vt.NewSafeEmulator(width, height)
	// Both copies end with an error when the PTY or emulator closes. That end
	// is expected, so the error is intentionally dropped.
	go func() { _, _ = io.Copy(emu, p) }() // shell output into the screen grid
	go func() { _, _ = io.Copy(p, emu) }() // encoded keys and terminal replies to the shell
	return &shell{pty: p, emu: emu, cmd: cmd}, nil
}

func (s *shell) resize(width, height int) error {
	s.emu.Resize(width, height)
	if err := s.pty.Resize(width, height); err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}
	return nil
}

// close ends the shell by closing the PTY. It does not call emu.Close: in vt,
// Close is not locked and races with the goroutine blocked in emu.Read, so
// that goroutine stays parked. Epic #50 must solve this in internal/term.
func (s *shell) close() {
	_ = s.pty.Close()
}
