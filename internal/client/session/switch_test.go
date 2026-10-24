package session

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// sessionsAt lists api (this screen's session), web, and old, as seen at the
// fixed clock.
func sessionsAt() *api.SessionList {
	now := fixedClock()
	return &api.SessionList{Sessions: []api.SessionInfo{
		{Name: "api", State: api.StateRunning, Attached: 1, Windows: 1, LastUsed: now.Add(-time.Minute)},
		{Name: "web", State: api.StateRunning, Windows: 2, LastUsed: now.Add(-3 * time.Hour)},
		{Name: "old", State: api.StateSaved, LastUsed: now.Add(-100 * time.Hour)},
	}}
}

// switchScreen is the three-pane session "api" that knows the session list.
func switchScreen(t *testing.T, cols, rows int) (*Model, *recorder) {
	t.Helper()
	m, r := pickerScreen(t, cols, rows)
	m.now = fixedClock
	m.Update(sessionsAt())
	return m, r
}

// openSessions presses Ctrl+b s and returns the command it gave.
func openSessions(m *Model) tea.Cmd {
	m.Update(ctrlB)
	return m.Update(press('s', 0, "s"))
}

func plain(m *Model) string { return ansi.Strip(m.View().Content) }

// switchOf runs cmd and returns the SwitchMsg it gives, if any.
func switchOf(cmd tea.Cmd) (SwitchMsg, bool) {
	if cmd == nil {
		return SwitchMsg{}, false
	}
	s, ok := cmd().(SwitchMsg)
	return s, ok
}

func TestSessionsKeyOpensList(t *testing.T) {
	m, r := switchScreen(t, 80, 24)
	if cmd := openSessions(m); cmd != nil {
		if _, ok := switchOf(cmd); ok {
			t.Fatal("Ctrl+b s switched at once; it should open the list")
		}
	}
	view := plain(m)
	for _, name := range []string{"Sessions", " api ", " web ", " old ", "current"} {
		if !strings.Contains(view, name) {
			t.Errorf("the list does not show %q:\n%s", name, view)
		}
	}
	if len(r.sent) != 0 {
		t.Fatalf("sent %v, want nothing", r.sent)
	}
}

func TestSessionListEnterSwitches(t *testing.T) {
	m, r := switchScreen(t, 80, 24)
	openSessions(m)
	m.Update(press(tea.KeyDown, 0, ""))
	s, ok := switchOf(m.Update(enter))
	if !ok || s != (SwitchMsg{Session: "web"}) {
		t.Fatalf("Enter gave %#v, want a switch to web", s)
	}
	if strings.Contains(plain(m), "Sessions") || len(r.sent) != 0 {
		t.Fatalf("the list stays open or sent %v after Enter", r.sent)
	}
}

func TestSessionListEnterOnCurrentCloses(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	openSessions(m)
	if s, ok := switchOf(m.Update(enter)); ok {
		t.Fatalf("Enter on the current session gave %#v, want nothing", s)
	}
	if strings.Contains(plain(m), "Sessions") {
		t.Fatal("the list stays open")
	}
}

func TestSessionListEscCloses(t *testing.T) {
	m, r := switchScreen(t, 80, 24)
	openSessions(m)
	if s, ok := switchOf(m.Update(press(tea.KeyEscape, 0, ""))); ok {
		t.Fatalf("Esc gave %#v, want nothing", s)
	}
	m.Update(press('l', 0, "l"))
	if strings.Contains(plain(m), "Sessions") {
		t.Fatal("the list stays open after Esc")
	}
	if got := r.keys(); len(r.sent) != 1 || got[0] != "l" {
		t.Fatalf("sent %v, want only the key l after Esc", r.sent)
	}
}

// TestSessionListFollowsUpdates: a session started elsewhere shows up while
// the list is open.
func TestSessionListFollowsUpdates(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	openSessions(m)
	l := sessionsAt()
	l.Sessions = append(l.Sessions, api.SessionInfo{Name: "docs", State: api.StateRunning, LastUsed: fixedClock()})
	m.Update(l)
	if !strings.Contains(plain(m), " docs ") {
		t.Fatal("the new session docs is not listed")
	}
}

func TestSessionListClickSwitches(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	openSessions(m)
	for y, row := range strings.Split(plain(m), "\n") {
		if x := strings.Index(row, " web "); x >= 0 {
			s, ok := switchOf(m.Update(click(x+1, y)))
			if !ok || s.Session != "web" {
				t.Fatalf("click on web gave %#v, want a switch to web", s)
			}
			return
		}
	}
	t.Fatal("web is not drawn")
}

func TestSessionListClickOutsideCloses(t *testing.T) {
	m, r := switchScreen(t, 80, 24)
	openSessions(m)
	if s, ok := switchOf(m.Update(click(0, 0))); ok {
		t.Fatalf("click outside gave %#v", s)
	}
	if strings.Contains(plain(m), "Sessions") || len(r.sent) != 0 {
		t.Fatalf("list open %v, sent %v; want it closed and nothing sent", strings.Contains(plain(m), "Sessions"), r.sent)
	}
}

// TestDetachStillGoesHome: Ctrl+b d asks to detach, as before #88.
func TestDetachStillGoesHome(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	m.Update(ctrlB)
	cmd := m.Update(press('d', 0, "d"))
	if cmd == nil {
		t.Fatal("Ctrl+b d did nothing")
	}
	if _, ok := cmd().(DetachMsg); !ok {
		t.Fatal("Ctrl+b d did not ask to detach")
	}
}

func goldenSessions(t *testing.T, cols, rows int) {
	m, _ := switchScreen(t, cols, rows)
	openSessions(m)
	m.Update(press(tea.KeyDown, 0, ""))
	golden.RequireEqual(t, m.View().Content)
}

func TestSessionListGolden80x24(t *testing.T)  { goldenSessions(t, 80, 24) }
func TestSessionListGolden120x40(t *testing.T) { goldenSessions(t, 120, 40) }

// TestMenuOpensSessionList: the right-click menu reaches the list too.
func TestMenuOpensSessionList(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	chooseByClick(t, m, "Switch session")
	if !strings.Contains(plain(m), " old ") {
		t.Fatal("Switch session in the menu did not open the list")
	}
}
