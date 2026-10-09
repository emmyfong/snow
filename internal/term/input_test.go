package term

import (
	"strings"
	"testing"
	"time"
)

// TestInputDoesNotHoldTheLock: a program that stops reading must not block
// readers of the screen, or one paste freezes every session on the server.
func TestInputDoesNotHoldTheLock(t *testing.T) {
	f := newFakePTY()
	f.stuck = make(chan struct{})
	p := newPane(f, 80, 24, Options{})
	t.Cleanup(func() { close(f.stuck); _ = p.Close() })

	pasted := make(chan struct{})
	go func() {
		p.Paste(strings.Repeat("x", 512*1024))
		for range 1000 {
			p.SendKey(Key{Code: 'a', Text: "a"})
		}
		close(pasted)
	}()

	select {
	case <-pasted:
	case <-time.After(2 * time.Second):
		t.Fatal("Paste blocked behind a program that is not reading")
	}
	done := make(chan struct{})
	go func() {
		_ = p.Lines()
		_, _ = p.Size()
		_ = p.Resize(100, 30)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reading the screen blocked while input was stuck")
	}
}

func TestInputArrivesInOrder(t *testing.T) {
	p, f := newTestPane(t)
	p.Paste("one ")
	p.SendKey(Key{Code: 't', Text: "t"})
	p.Paste(" three")
	eventually(t, "input in order", func() bool { return f.sent() == "one t three" })
}

func TestResizeChangesVersion(t *testing.T) {
	p, _ := newTestPane(t)
	before := p.Version()
	if err := p.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if p.Version() == before {
		t.Fatal("Resize changed the screen but not Version, so clients keep stale rows")
	}
}
