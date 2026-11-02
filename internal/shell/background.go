package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Process is a program started in the background, with every process it
// starts: a process group on Unix, a job object on Windows. It runs with
// no window and no terminal until Stop.
type Process struct {
	cmd  *exec.Cmd
	tree *tree
	done chan struct{}
	err  error // how the program ended; read after done closes
}

// StartBackground starts l with stdin closed and its output, stdout and
// stderr together, written to out. On Windows it starts with
// CREATE_NO_WINDOW, so the programs it runs open no window either.
func StartBackground(l Launch, out io.Writer) (*Process, error) {
	cmd, tree, err := l.start(context.Background(), out)
	if err != nil {
		return nil, err
	}
	p := &Process{cmd: cmd, tree: tree, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// Done is closed when the program exits. Processes it started may still
// run until Stop.
func (p *Process) Done() <-chan struct{} { return p.done }

// Err returns how the program ended: nil for exit code 0. Call it after
// Done is closed.
func (p *Process) Err() error { return p.err }

// Stop ends the program and every process it started. On Unix they get
// SIGTERM first and grace to exit, so a server can save its state; then
// whatever runs is killed. On Windows the job ends them at once. Stop
// returns when the program has exited.
func (p *Process) Stop(grace time.Duration) error {
	var errs []error
	if err := p.tree.terminate(); err != nil {
		errs = append(errs, fmt.Errorf("stop: %w", err))
	}
	select {
	case <-p.done:
	case <-time.After(grace):
	}
	if err := p.tree.kill(); err != nil {
		errs = append(errs, fmt.Errorf("kill: %w", err))
	}
	<-p.done
	p.tree.close()
	return errors.Join(errs...)
}
