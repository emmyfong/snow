package term

import (
	"context"
	"errors"
	"time"
)

// ErrNoExit means a killed program did not exit in time. Its process may
// still exist; the pane closes anyway.
var ErrNoExit = errors.New("term: program did not exit after being killed")

// killWait bounds how long Close waits for a killed program to exit. A
// program stuck in the kernel (seen once on a macOS runner) must not hang
// Close, or the server with it.
const killWait = 5 * time.Second

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
	case <-done:
		return ctx.Err()
	case <-time.After(limit):
		return ErrNoExit
	}
}
