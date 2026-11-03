//go:build windows

package tool

import (
	"context"
	"errors"
	"testing"
)

// TestResolveWSLPathsCanceled: a canceled context stops the resolve, so a
// turn the user cancels does not wait for wsl.exe.
func TestResolveWSLPathsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolveWSLPaths(ctx, "NoSuchDistro", "/")
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
}
