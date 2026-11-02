package agents

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// waiting is a list with two agents that need the user, the Claude Code one
// first, and one that works.
func waiting() []api.AgentInfo {
	return []api.AgentInfo{
		{Session: "web", Window: 0, WindowName: "sh", Pane: 9, Kind: api.AgentKindClaudeCode, State: api.AgentNeedsYou, Since: base.Add(-time.Hour)},
		{Session: "api", Window: 1, WindowName: "dev", Pane: 2, Kind: api.AgentKindChat, State: api.AgentNeedsYou, Since: base.Add(-3 * time.Minute), Waits: api.AgentWaitsQuestion},
		{Session: "api", Window: 2, WindowName: "review", Pane: 7, Kind: api.AgentKindClaudeCode, State: api.AgentWorking, Since: base},
	}
}

func inbox() *Inbox {
	in := NewInbox(clock())
	in.SetAgents(waiting())
	return in
}

// TestInboxListsNeedsYouOldestFirst: only agents that wait for the user are
// listed, in every session, the one that waited longest first.
func TestInboxListsNeedsYouOldestFirst(t *testing.T) {
	in := inbox()
	s := ansi.Strip(func() string { box, _, _ := in.View(theme.Default(), 120, 40); return box }())
	if !strings.Contains(s, "2 agents need you") {
		t.Errorf("inbox summary:\n%s", s)
	}
	if web, api := strings.Index(s, "web"), strings.Index(s, "api"); web < 0 || api < 0 || web > api {
		t.Errorf("inbox order:\n%s", s)
	}
	if strings.Contains(s, "review") || strings.Contains(s, "working") {
		t.Errorf("inbox lists an agent that does not need the user:\n%s", s)
	}
	for _, want := range []string{"1 dev", "chat", "question", "3m", "Claude Code", "waiting", "1h"} {
		if !strings.Contains(s, want) {
			t.Errorf("inbox lacks %q:\n%s", want, s)
		}
	}
}

// TestInboxFollowsTheServer: an answered agent leaves the inbox, and the
// selection moves to the oldest agent left.
func TestInboxFollowsTheServer(t *testing.T) {
	in := inbox()
	a, ok := in.Choice()
	if !ok || a.Pane != 9 {
		t.Fatalf("selected %+v %v, want the oldest, pane 9", a, ok)
	}
	in.SetAgents(waiting()[1:])
	a, ok = in.Choice()
	if !ok || a.Pane != 2 {
		t.Fatalf("selected %+v %v after the answer, want pane 2", a, ok)
	}
	in.SetAgents(nil)
	if _, ok := in.Choice(); ok {
		t.Fatal("an empty inbox still has a choice")
	}
	if r := in.Key(key(tea.KeyEnter)); r != picker.Cancelled {
		t.Fatalf("Enter on an empty inbox gave %v, want Cancelled", r)
	}
	if box, _, _ := in.View(theme.Default(), 80, 24); !strings.Contains(ansi.Strip(box), "No agent needs you.") {
		t.Fatalf("empty inbox:\n%s", ansi.Strip(box))
	}
}

// TestInboxKeepsTheSelection: a new list keeps the selection on the same
// agent while it waits.
func TestInboxKeepsTheSelection(t *testing.T) {
	in := inbox()
	in.Key(key(tea.KeyDown))
	list := waiting()
	list[1].Since = list[1].Since.Add(-time.Hour) // now older than the first
	in.SetAgents(list)
	if a, _ := in.Choice(); a.Pane != 2 {
		t.Fatalf("selection moved to pane %d, want it kept on 2", a.Pane)
	}
}

// TestInboxKey: arrows move, Esc closes.
func TestInboxKey(t *testing.T) {
	in := inbox()
	if r := in.Key(key(tea.KeyDown)); r != picker.Open {
		t.Fatalf("Down gave %v, want Open", r)
	}
	if a, _ := in.Choice(); a.Pane != 2 {
		t.Fatalf("selected pane %d, want 2", a.Pane)
	}
	if r := in.Key(key(tea.KeyEscape)); r != picker.Cancelled {
		t.Fatalf("Esc gave %v, want Cancelled", r)
	}
}

// TestInboxGolden draws the inbox at two sizes in a dark and a light theme.
func TestInboxGolden(t *testing.T) {
	for _, th := range []theme.Theme{theme.Default(), lightTheme()} {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", th.Name, size[0], size[1]), func(t *testing.T) {
				box, x, y := inbox().View(th, size[0], size[1])
				if box == "" {
					t.Fatal("no inbox drawn")
				}
				if w, _ := sizeOf(box); w > size[0] {
					t.Fatalf("inbox is %d wide, the window %d", w, size[0])
				}
				if x < 0 || y < 0 {
					t.Fatalf("inbox at %d,%d, outside the window", x, y)
				}
				golden.RequireEqual(t, box)
			})
		}
	}
}
