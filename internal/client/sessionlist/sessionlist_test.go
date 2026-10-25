package sessionlist

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func key(code rune, text string) tea.Key { return tea.Key{Code: code, Text: text} }

// three is a list of api (current), web, and old, newest first.
func three(current string) *Model {
	m := New(current, func() time.Time { return base })
	m.SetSessions([]api.SessionInfo{info("old", 50*time.Hour, 0), info("api", time.Minute, 1), info("web", time.Hour, 0)})
	return m
}

func TestPreselectsCurrent(t *testing.T) {
	if got := three("web").Choice(); got != "web" {
		t.Fatalf("selected %q, want the current session web", got)
	}
	if got := three("gone").Choice(); got != "api" {
		t.Fatalf("selected %q, want the first row when the current session is not listed", got)
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name   string
		keys   []tea.Key
		result Result
		choice string
	}{
		{"down moves", []tea.Key{key(tea.KeyDown, "")}, Open, "web"},
		{"j moves", []tea.Key{key('j', "j")}, Open, "web"},
		{"up wraps", []tea.Key{key(tea.KeyUp, "")}, Open, "old"},
		{"k wraps", []tea.Key{key('k', "k")}, Open, "old"},
		{"enter chooses", []tea.Key{key(tea.KeyDown, ""), key(tea.KeyEnter, "")}, Chosen, "web"},
		{"esc closes", []tea.Key{key(tea.KeyEscape, "")}, Closed, "api"},
		{"other keys do nothing", []tea.Key{key('x', "x")}, Open, "api"},
	}
	for _, tt := range tests {
		m := three("api")
		var r Result
		for _, k := range tt.keys {
			r = m.Key(k)
		}
		if r != tt.result || m.Choice() != tt.choice {
			t.Errorf("%s: result %v choice %q, want %v %q", tt.name, r, m.Choice(), tt.result, tt.choice)
		}
	}
}

func TestEmptyListCloses(t *testing.T) {
	m := New("api", time.Now)
	if r := m.Key(key(tea.KeyEnter, "")); r != Closed {
		t.Fatalf("Enter on an empty list gave %v, want Closed", r)
	}
	if box, _, _ := m.View(theme.Default(), 80, 23); !strings.Contains(ansi.Strip(box), "No sessions") {
		t.Fatalf("empty list drew %q, want a note", ansi.Strip(box))
	}
}

// TestSelectionFollowsUpdates: a new list in another order keeps the
// highlight on the same session.
func TestSelectionFollowsUpdates(t *testing.T) {
	m := three("api")
	m.Key(key(tea.KeyDown, "")) // web
	m.SetSessions([]api.SessionInfo{info("web", time.Second, 1), info("api", time.Minute, 1)})
	if got := m.Choice(); got != "web" {
		t.Fatalf("selected %q after the list changed, want web", got)
	}
	m.SetSessions([]api.SessionInfo{info("api", time.Minute, 1)})
	if got := m.Choice(); got != "api" {
		t.Fatalf("selected %q after web ended, want api", got)
	}
}

// rowY returns the screen row where the popup draws session name.
func rowY(t *testing.T, m *Model, name string) (x, y int) {
	t.Helper()
	box, bx, by := m.View(theme.Default(), 80, 23)
	for i, row := range strings.Split(ansi.Strip(box), "\n") {
		if strings.Contains(row, " "+name+" ") {
			return bx + 3, by + i
		}
	}
	t.Fatalf("%s not drawn in %q", name, ansi.Strip(box))
	return 0, 0
}

func TestClick(t *testing.T) {
	th := theme.Default()
	m := three("api")
	x, y := rowY(t, m, "old")
	if r := m.Click(th, 80, 23, x, y); r != Chosen || m.Choice() != "old" {
		t.Fatalf("click on old gave %v %q, want Chosen old", r, m.Choice())
	}
	m = three("api")
	_, bx, by := m.View(th, 80, 23)
	if r := m.Click(th, 80, 23, bx, by); r != Open {
		t.Fatalf("click on the frame gave %v, want Open", r)
	}
	if r := m.Click(th, 80, 23, 0, 0); r != Closed {
		t.Fatalf("click outside gave %v, want Closed", r)
	}
}

// TestCurrentIsMarked: the session the screen shows says so.
func TestCurrentIsMarked(t *testing.T) {
	box, _, _ := three("api").View(theme.Default(), 80, 23)
	for _, row := range strings.Split(ansi.Strip(box), "\n") {
		if strings.Contains(row, " api ") && !strings.Contains(row, "current") {
			t.Fatalf("row %q does not mark api as current", row)
		}
		if strings.Contains(row, " web ") && strings.Contains(row, "current") {
			t.Fatalf("row %q marks web as current", row)
		}
	}
}

// TestViewIsWidthSafe: the popup never draws past the screen, and a tiny
// screen draws nothing instead of panicking.
func TestViewIsWidthSafe(t *testing.T) {
	th := theme.Default()
	m := New("api", func() time.Time { return base })
	var many []api.SessionInfo
	for i := range 30 {
		many = append(many, info("session-with-a-long-name-"+string(rune('a'+i%26)), time.Duration(i)*time.Minute, i%3))
	}
	m.SetSessions(many)
	for _, sz := range [][2]int{{0, 0}, {3, 3}, {10, 4}, {20, 6}, {80, 23}, {120, 39}} {
		box, x, y := m.View(th, sz[0], sz[1])
		if box == "" {
			continue
		}
		w, h := ui.Size(box)
		if x < 0 || y < 0 || x+w > sz[0] || y+h > sz[1] {
			t.Errorf("%dx%d: box %dx%d at %d,%d leaves the screen", sz[0], sz[1], w, h, x, y)
		}
	}
	for range 29 {
		m.Key(key(tea.KeyDown, ""))
	}
	box, _, _ := m.View(th, 80, 12)
	if !strings.Contains(ansi.Strip(box), m.Choice()) {
		t.Fatal("the selected session scrolled out of view")
	}
}
