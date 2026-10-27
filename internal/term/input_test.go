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

// TestSendKeyShiftedText: vt writes a printable key only when it has no
// modifier, so Shift+a and the other shapes of an uppercase key reached the
// program as nothing (#236). Each key is followed by z, so the test knows the
// pane has sent it.
func TestSendKeyShiftedText(t *testing.T) {
	tests := []struct {
		name string
		key  Key
		want string
	}{
		// Legacy "A", kitty ESC[97;2u, and xterm ESC[27;2;65~ all decode
		// to this shape.
		{"shift+a", Key{Code: 'a', Mod: ModShift, Text: "A"}, "A"},
		// Kitty ESC[65u: the client lowercases the code and keeps the text.
		{"text without shift", Key{Code: 'a', Text: "A"}, "A"},
		{"shift+non-ASCII", Key{Code: 'é', Mod: ModShift, Text: "É"}, "É"},
		{"shift+space", Key{Code: ' ', Mod: ModShift, Text: " "}, " "},
		// Terminals send Alt+Shift+a with no text.
		{"alt+shift+a", Key{Code: 'a', Mod: ModAlt | ModShift}, "\x1bA"},
		{"alt+a", Key{Code: 'a', Mod: ModAlt}, "\x1ba"},
		{"ctrl+c", Key{Code: 'c', Mod: ModCtrl}, "\x03"},
		{"shift+tab", Key{Code: KeyTab, Mod: ModShift}, "\x1b[Z"},
		{"enter", Key{Code: KeyEnter}, "\r"},
		// Keys the text path must leave to vt.
		{"special key with text", Key{Code: KeyUp, Text: "x"}, "\x1b[A"},
		{"text with control chars", Key{Code: 'a', Text: "a\x1b"}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPane(t)
			p.SendKey(tt.key)
			p.SendKey(Key{Code: 'z', Text: "z"})
			eventually(t, "the marker key", func() bool { return strings.HasSuffix(f.sent(), "z") })
			if got := f.sent(); got != tt.want+"z" {
				t.Fatalf("SendKey(%+v) sent %q, want %q", tt.key, strings.TrimSuffix(got, "z"), tt.want)
			}
		})
	}
}
