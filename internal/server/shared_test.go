package server

import (
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// paneSize returns the pane rectangle in the next Layout c receives.
func paneSize(t *testing.T, c *apitest.Client) (int, int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil {
			continue
		}
		if l, ok := m.(*api.Layout); ok {
			r := l.Windows[0].Panes[0].Rect
			return r.W, r.H
		}
	}
	t.Fatal("no Layout received")
	return 0, 0
}

func TestTwoClientsSameScreen(t *testing.T) {
	s := newTestServer(t)
	a := connect(t, s, 100, 30)
	a.Send(&api.Attach{Session: "pair", Create: true})
	a.WaitScreen(strings.TrimRight(prompt(), " "))
	a.Send(&api.Input{Pane: 1, Paste: "echo before\r"})
	a.WaitScreen("\nbefore\n")

	// b joins late: it must get the whole screen, not only later changes.
	b := connect(t, s, 100, 30)
	b.Send(&api.Attach{Session: "pair"})
	b.WaitScreen("\nbefore\n")

	a.Send(&api.Input{Pane: 1, Paste: "echo shared\r"})
	a.WaitScreen("\nshared\n")
	b.WaitScreen("\nshared\n")
	a.Drain(300 * time.Millisecond)
	b.Drain(300 * time.Millisecond)
	if a.Screen() != b.Screen() {
		t.Fatalf("screens differ:\na:\n%s\nb:\n%s", a.Screen(), b.Screen())
	}
}

// TestReattachShowsFullScreen: a client that attaches after the only other
// client left sees everything the session printed before.
func TestReattachShowsFullScreen(t *testing.T) {
	s := newTestServer(t)
	a := connect(t, s, 80, 24)
	a.Send(&api.Attach{Session: "work", Create: true})
	a.WaitScreen(strings.TrimRight(prompt(), " "))
	a.Send(&api.Input{Pane: 1, Paste: "echo earlier\r"})
	a.WaitScreen("\nearlier\n")
	a.Close()

	b := connect(t, s, 80, 24)
	b.Send(&api.Attach{Session: "work"})
	b.WaitScreen("\nearlier\n")
}

func TestSmallestSizeWins(t *testing.T) {
	s := newTestServer(t)
	big := connect(t, s, 120, 40)
	big.Send(&api.Attach{Session: "pair", Create: true})
	if w, h := paneSize(t, big); w != 120 || h != 40 {
		t.Fatalf("alone: pane %dx%d, want 120x40", w, h)
	}
	small := connect(t, s, 80, 24)
	small.Send(&api.Attach{Session: "pair"})
	if w, h := paneSize(t, big); w != 80 || h != 24 {
		t.Fatalf("with a smaller client: pane %dx%d, want 80x24", w, h)
	}
	if cols, rows := s.pane(t, "pair").Size(); cols != 80 || rows != 24 {
		t.Fatalf("pane is %dx%d, want 80x24", cols, rows)
	}

	// The smaller client leaves: the session grows back.
	small.Close()
	if w, h := paneSize(t, big); w != 120 || h != 40 {
		t.Fatalf("after the small client left: pane %dx%d, want 120x40", w, h)
	}
}
