package term

import (
	"strings"
	"testing"
	"time"
)

// startReal runs spec on a real PTY for an OS-specific test.
func startReal(t *testing.T, spec Spec) *Pane {
	t.Helper()
	p, err := Start(spec, 80, 24, Options{})
	if err != nil {
		t.Fatalf("start %s: %v", spec.Path, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// waitForPrompt waits until the shell shows its prompt. Shells discard input
// that arrives while they start, so tests must not type before this.
func waitForPrompt(t *testing.T, p *Pane, prompt string) {
	t.Helper()
	waitForCount(t, p, prompt, 1)
}

// typeLine types text and Enter through the emulator, the path real keys take.
func typeLine(p *Pane, text string) {
	for _, r := range text {
		p.SendKey(Key{Code: r, Text: string(r)})
	}
	p.SendKey(Key{Code: KeyEnter})
}

// waitForCount waits until text appears n times on screen. A typed command
// shows once on the command line, so n=2 means the program printed it too.
func waitForCount(t *testing.T, p *Pane, text string, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(p.Text(), text) >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%q did not appear %d times; screen:\n%s", text, n, p.Text())
}

func waitDone(t *testing.T, p *Pane, what string) {
	t.Helper()
	select {
	case <-p.Done():
	case <-time.After(15 * time.Second):
		t.Fatalf("%s: program did not exit; screen:\n%s", what, p.Text())
	}
}
