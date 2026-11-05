package session

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// unknownScreen is a session with a shell on the left and a pane of a kind
// this client does not know on the right, focused.
func unknownScreen(t *testing.T, cols, rows int) *Model {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(cols, rows)
	left := cols / 2
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Rect: api.Rect{W: left, H: rows - 1}, Profile: "sh"},
		{ID: 2, Kind: "files", Rect: api.Rect{X: left + 1, W: cols - left - 1, H: rows - 1}, Focused: true},
	}}}})
	m.Update(&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 0, Text: "$ ls"}}})
	return m
}

func unknownKindGolden(t *testing.T, cols, rows int) {
	m := unknownScreen(t, cols, rows)
	// State for the unknown kind must not break the client.
	m.Update(&api.SidePaneState{Pane: 2, Kind: "files", Rev: 1, State: json.RawMessage(`{"root":"/src"}`)})
	golden.RequireEqual(t, m.View().Content)
}

func TestUnknownKindPlaceholderGolden80x24(t *testing.T)  { unknownKindGolden(t, 80, 24) }
func TestUnknownKindPlaceholderGolden120x40(t *testing.T) { unknownKindGolden(t, 120, 40) }

// TestOwnPaneKinds: a terminal gets no own pane, chat gets a chat, and any
// other kind gets the placeholder.
func TestOwnPaneKinds(t *testing.T) {
	m := New((&recorder{}).send)
	tests := []struct {
		kind string
		want string
	}{
		{api.PaneTerminal, "<nil>"},
		{"", "<nil>"},
		{api.PaneChat, "session.chatPane"},
		{"files", "*session.unknownPane"},
	}
	for _, tt := range tests {
		got := "<nil>"
		if p := m.newOwnPane(tt.kind, 3); p != nil {
			got = typeName(p)
		}
		if got != tt.want {
			t.Errorf("newOwnPane(%q) = %s, want %s", tt.kind, got, tt.want)
		}
	}
}

// TestUnknownPaneTiny: the placeholder draws nothing wider than its pane,
// down to one cell, and nothing at zero.
func TestUnknownPaneTiny(t *testing.T) {
	u := newUnknownPane("files", New((&recorder{}).send).theme)
	for _, w := range []int{0, 1, 5, 12} {
		u.SetSize(w, 3)
		for _, row := range u.View() {
			if got := stringWidth(row); got > w {
				t.Fatalf("width %d: row %q is %d wide", w, row, got)
			}
		}
	}
}

// TestPaneKindsKept: the session keeps the server's kind list.
func TestPaneKindsKept(t *testing.T) {
	m := New((&recorder{}).send)
	want := []api.PaneKindInfo{{Name: api.PaneTerminal, Title: "Terminal"}, {Name: "files", Title: "Files"}}
	m.Update(&api.PaneKinds{Kinds: want})
	if len(m.kinds) != 2 || m.kinds[1] != want[1] {
		t.Fatalf("kinds %+v, want %+v", m.kinds, want)
	}
}

func typeName(v any) string { return fmt.Sprintf("%T", v) }

func stringWidth(s string) int { return ansi.StringWidth(s) }
