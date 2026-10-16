package session

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func hintShown(m *Model) bool { return strings.Contains(m.View().Content, "Split right") }

// TestWhichKeyDelayed: a fast second key never shows the popup, and the
// timer of an earlier prefix press never shows it for a later one.
func TestWhichKeyDelayed(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	first := m.prefixSeq
	m.Update(press('z', 0, "z"))
	m.Update(hintMsg{first}) // the timer fires after the key already arrived
	if hintShown(m) {
		t.Fatal("the popup showed after a fast second key")
	}
	if c := lastCommand(t, r); c.Op != api.OpZoom {
		t.Fatalf("sent %+v, want zoom", c)
	}

	m.Update(ctrlB)
	m.Update(hintMsg{first}) // a stale timer, from the first press
	if hintShown(m) {
		t.Fatal("a stale timer showed the popup for a later prefix press")
	}
}

func TestWhichKeyShowsAfterThePause(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.hintDelay = time.Millisecond
	cmd := m.Update(ctrlB)
	if cmd == nil {
		t.Fatal("the prefix started no timer")
	}
	if hintShown(m) {
		t.Fatal("the popup showed before the pause")
	}
	m.Update(cmd()) // the real timer, after one millisecond
	if !hintShown(m) {
		t.Fatalf("no popup after the pause:\n%s", m.View().Content)
	}
	m.Update(press('z', 0, "z"))
	if hintShown(m) {
		t.Fatal("the popup stayed after a key")
	}
	if c := lastCommand(t, r); c.Op != api.OpZoom {
		t.Fatalf("the key that closed the popup sent %+v, want zoom", c)
	}
}

func TestWhichKeyClosesOnUnboundKeyAndDoesNotSendIt(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(hintMsg{m.prefixSeq})
	m.Update(press('q', 0, "q"))
	if hintShown(m) || len(r.sent) != 0 {
		t.Fatalf("popup shown %v, sent %v; want it closed and nothing sent", hintShown(m), r.sent)
	}
}

func TestWhichKeyShowsRebinding(t *testing.T) {
	m, _ := newPrefixScreen(t)
	m.keys = rebound(t)
	m.Update(ctrlB)
	m.Update(hintMsg{m.prefixSeq})
	if rows := m.View().Content; !strings.Contains(rows, "|") || strings.Contains(rows, "%") {
		t.Fatalf("popup does not show the new key:\n%s", rows)
	}
}

func hintGolden(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	m.Update(ctrlB)
	m.Update(hintMsg{m.prefixSeq})
	golden.RequireEqual(t, m.View().Content)
}

func TestWhichKeyGolden80x24(t *testing.T)  { hintGolden(t, 80, 24) }
func TestWhichKeyGolden120x40(t *testing.T) { hintGolden(t, 120, 40) }

// TestWhichKeyTinyScreen: a popup that cannot fit is cut, never a panic.
func TestWhichKeyTinyScreen(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {8, 4}, {3, 3}, {1, 1}} {
		m := screen(size[0], size[1])
		panes := []api.PaneInfo{{ID: 1, Rect: api.Rect{W: size[0], H: max(size[1]-1, 0)}, Focused: true}}
		m.Update(&api.Layout{Session: "s", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: panes}}})
		m.Update(ctrlB)
		m.Update(hintMsg{m.prefixSeq})
		for _, row := range strings.Split(m.View().Content, "\n") {
			if w := ansi.StringWidth(row); w > size[0] {
				t.Errorf("screen %v: a row is %d wide", size, w)
			}
		}
	}
}
