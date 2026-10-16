package term

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestWaitGivesUpOnAProgramThatWillNotDie: if a killed program does not exit,
// closing its pane must not hang forever, as it did on a macOS CI runner.
func TestWaitGivesUpOnAProgramThatWillNotDie(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	killed := false
	start := time.Now()
	err := waitOrKill(ctx, func() error { select {} }, func() { killed = true }, 50*time.Millisecond)
	if !killed {
		t.Fatal("the program was not killed")
	}
	if !errors.Is(err, ErrNoExit) {
		t.Fatalf("error %v, want ErrNoExit", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("waited far past the limit")
	}
}

func TestWaitReturnsTheExit(t *testing.T) {
	boom := errors.New("boom")
	if err := waitOrKill(context.Background(), func() error { return boom }, func() {}, time.Second); !errors.Is(err, boom) {
		t.Fatalf("error %v, want the program's exit error", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exited := make(chan struct{})
	err := waitOrKill(ctx, func() error { <-exited; return nil }, func() { close(exited) }, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("killed program: error %v, want context.Canceled", err)
	}
}
