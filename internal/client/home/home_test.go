package home

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var now = time.Date(2026, 10, 15, 14, 2, 0, 0, time.UTC)

func screen(cols, rows int, sessions ...api.SessionInfo) *Model {
	m := New("v0.1.0", theme.Default())
	m.now = func() time.Time { return now }
	m.SetSize(cols, rows)
	m.Update(&api.SessionList{Sessions: sessions})
	return m
}

func session(name string, ago time.Duration, attached int) api.SessionInfo {
	return api.SessionInfo{Name: name, Windows: 1, Attached: attached, State: api.StateRunning, LastUsed: now.Add(-ago)}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
}

func press(code rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: code}) }

// open runs cmd and returns the OpenMsg it produces.
func open(t *testing.T, cmd tea.Cmd) OpenMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	o, ok := cmd().(OpenMsg)
	if !ok {
		t.Fatalf("command produced %T, want OpenMsg", cmd())
	}
	return o
}

func TestEnterAttachesOrCreates(t *testing.T) {
	tests := []struct {
		name  string
		typed string
		want  OpenMsg
	}{
		{"existing session attaches", "api", OpenMsg{Session: "api"}},
		{"new name creates", "logs", OpenMsg{Session: "logs", Create: true}},
		{"empty starts a numbered session", "", OpenMsg{Create: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := screen(80, 24, session("api", time.Minute, 1), session("dotfiles", time.Hour, 0))
			typeText(m, tt.typed)
			if got := open(t, m.Update(press(tea.KeyEnter))); got != tt.want {
				t.Fatalf("Enter after %q opened %+v, want %+v", tt.typed, got, tt.want)
			}
		})
	}
}

func TestArrowsSelectASession(t *testing.T) {
	m := screen(80, 24, session("old", 48*time.Hour, 0), session("new", time.Minute, 0))
	m.Update(press(tea.KeyDown)) // the most recent session is listed first
	if got := open(t, m.Update(press(tea.KeyEnter))); got != (OpenMsg{Session: "new"}) {
		t.Fatalf("Down then Enter opened %+v, want the most recent session", got)
	}
	m = screen(80, 24, session("old", 48*time.Hour, 0), session("new", time.Minute, 0))
	m.Update(press(tea.KeyDown))
	m.Update(press(tea.KeyDown))
	if got := open(t, m.Update(press(tea.KeyEnter))); got != (OpenMsg{Session: "old"}) {
		t.Fatalf("Down twice then Enter opened %+v, want old", got)
	}
}

func TestTypingFiltersTheList(t *testing.T) {
	m := screen(80, 24, session("api-server", time.Minute, 0), session("dotfiles", time.Hour, 0))
	typeText(m, "dot")
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "dotfiles") || strings.Contains(view, "api-server") {
		t.Fatalf("after typing dot the list shows:\n%s", view)
	}
	m.Update(press(tea.KeyBackspace))
	m.Update(press(tea.KeyBackspace))
	m.Update(press(tea.KeyBackspace))
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "api-server") {
		t.Fatalf("after clearing the input the list shows:\n%s", view)
	}
}

func TestEscapeClearsThenQuits(t *testing.T) {
	m := screen(80, 24)
	typeText(m, "x")
	if cmd := m.Update(press(tea.KeyEscape)); cmd != nil {
		t.Fatal("Esc with text should only clear it")
	}
	cmd := m.Update(press(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("Esc on an empty input should quit")
	}
	if _, ok := cmd().(QuitMsg); !ok {
		t.Fatalf("Esc produced %T, want QuitMsg", cmd())
	}
}

func TestClickOpensASession(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 1), session("dotfiles", time.Hour, 0))
	view := ansi.Strip(m.View().Content)
	var y, x int
	for i, row := range strings.Split(view, "\n") {
		if c := strings.Index(row, "dotfiles"); c >= 0 {
			y, x = i, c
		}
	}
	cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if got := open(t, cmd); got != (OpenMsg{Session: "dotfiles"}) {
		t.Fatalf("click opened %+v, want dotfiles", got)
	}
}

func goldenWith(t *testing.T, cols, rows, n int) {
	var sessions []api.SessionInfo
	names := []string{"api-server", "dotfiles", "snow"}
	for i := range n {
		name := fmt.Sprintf("session-%02d", i)
		if i < len(names) {
			name = names[i]
		}
		sessions = append(sessions, session(name, time.Duration(i*i+1)*17*time.Minute, i%2))
	}
	golden.RequireEqual(t, screen(cols, rows, sessions...).View().Content)
}

func TestHomeNoSessions80x24(t *testing.T)      { goldenWith(t, 80, 24, 0) }
func TestHomeNoSessions120x40(t *testing.T)     { goldenWith(t, 120, 40, 0) }
func TestHomeThreeSessions80x24(t *testing.T)   { goldenWith(t, 80, 24, 3) }
func TestHomeThreeSessions120x40(t *testing.T)  { goldenWith(t, 120, 40, 3) }
func TestHomeTwentySessions80x24(t *testing.T)  { goldenWith(t, 80, 24, 20) }
func TestHomeTwentySessions120x40(t *testing.T) { goldenWith(t, 120, 40, 20) }

func TestAgo(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{30 * time.Hour, "yesterday"},
		{100 * time.Hour, "4 days ago"},
	}
	for _, tt := range tests {
		if got := ago(tt.d); got != tt.want {
			t.Errorf("ago(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
