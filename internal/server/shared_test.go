package server

import (
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// paneSize returns the pane rectangle in the next Layout c receives.
func (c *testClient) paneSize() (int, int) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m, err := c.next(time.Second)
		if err != nil {
			continue
		}
		if l, ok := m.(*api.Layout); ok {
			r := l.Windows[0].Panes[0].Rect
			return r.W, r.H
		}
	}
	c.t.Fatal("no Layout received")
	return 0, 0
}

func TestTwoClientsSameScreen(t *testing.T) {
	s := newTestServer(t)
	a := connect(t, s, 100, 30)
	a.send(&api.Attach{Session: "pair", Create: true})
	a.waitScreen(strings.TrimRight(prompt(), " "))
	b := connect(t, s, 100, 30)
	b.send(&api.Attach{Session: "pair"})
	b.waitScreen(strings.TrimRight(prompt(), " "))

	a.send(&api.Input{Pane: 1, Paste: "echo shared\r"})
	a.waitScreen("\nshared\n")
	b.waitScreen("\nshared\n")
}

func TestSmallestSizeWins(t *testing.T) {
	s := newTestServer(t)
	big := connect(t, s, 120, 40)
	big.send(&api.Attach{Session: "pair", Create: true})
	if w, h := big.paneSize(); w != 120 || h != 40 {
		t.Fatalf("alone: pane %dx%d, want 120x40", w, h)
	}
	small := connect(t, s, 80, 24)
	small.send(&api.Attach{Session: "pair"})
	if w, h := big.paneSize(); w != 80 || h != 24 {
		t.Fatalf("with a smaller client: pane %dx%d, want 80x24", w, h)
	}
	if cols, rows := s.pane(t, "pair").Size(); cols != 80 || rows != 24 {
		t.Fatalf("pane is %dx%d, want 80x24", cols, rows)
	}

	// The smaller client leaves: the session grows back.
	small.close()
	if w, h := big.paneSize(); w != 120 || h != 40 {
		t.Fatalf("after the small client left: pane %dx%d, want 120x40", w, h)
	}
}
