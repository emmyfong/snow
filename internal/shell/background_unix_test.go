//go:build !windows

package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStartBackgroundStopsGracefully: on Unix, Stop sends SIGTERM first,
// so a server can save its state before it exits.
func TestStartBackgroundStopsGracefully(t *testing.T) {
	dir := t.TempDir()
	bye, ready := filepath.Join(dir, "bye"), filepath.Join(dir, "ready")
	line := `trap 'echo bye > ` + bye + `; exit 0' TERM; echo > ` + ready + `; while true; do sleep 0.1; done`
	p, err := StartBackground(local(t).Command(dir, line), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	waitGrow(t, ready, 0)
	if err := p.Stop(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(bye); err != nil || string(b) != "bye\n" {
		t.Fatalf("the program did not get SIGTERM: %q, %v", b, err)
	}
}
