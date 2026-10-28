package term

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrNoExit means a killed program did not exit in time. Its process may
// still exist; the pane closes anyway.
var ErrNoExit = errors.New("term: program did not exit after being killed")

// ExitError is a program's non-zero exit status. A non-zero status is how a
// program ended, not a failure of the pane.
type ExitError struct{ Code int }

// Error implements error.
func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// exitErr returns what ended the program: nil for status 0, an ExitError for
// another status, and a wrapped failure otherwise.
func exitErr(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code >= 0 {
			return &ExitError{Code: code}
		}
	}
	return fmt.Errorf("wait: %w", err)
}

// waitOrKill waits for wait to return. When ctx ends first, it calls kill and
// waits at most limit for wait to return; after that it gives up with
// ErrNoExit and leaves wait running.
func waitOrKill(ctx context.Context, wait func() error, kill func(), limit time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	kill()
	select {
	case err := <-done:
		// The killed program's exit error only says it was killed; the
		// cancellation is why it stopped.
		_ = err
		return ctx.Err()
	case <-time.After(limit):
		return ErrNoExit
	}
}
