// Package sessionlist draws the sessions as list rows, for the home page,
// and is the session list popup the session screen opens to switch to
// another session, as tmux's choose-tree does.
package sessionlist

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Result is what a key press or a click did to the popup.
type Result int

// Results.
const (
	Open   Result = iota // the popup stays open
	Chosen               // Choice holds the chosen session
	Closed               // the user closed the popup
)

// Model is one open session list popup.
type Model struct {
	sessions []api.SessionInfo // most recently used first
	current  string            // the session the screen shows
	selected string            // the highlighted session's name
	now      func() time.Time
}

// New opens a popup with no sessions yet. Current is the session the
// screen shows: it is marked, and highlighted until the user moves.
func New(current string, now func() time.Time) *Model {
	return &Model{current: current, selected: current, now: now}
}

// SetSessions shows a new list. The highlight stays on the same session;
// when that one is gone, it moves to the current session, else the first.
func (m *Model) SetSessions(sessions []api.SessionInfo) {
	m.sessions = Sort(sessions)
	if m.index(m.selected) >= 0 {
		return
	}
	m.selected = m.current
	if m.index(m.selected) < 0 && len(m.sessions) > 0 {
		m.selected = m.sessions[0].Name
	}
}

func (m *Model) index(name string) int {
	return slices.IndexFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == name })
}

// Choice returns the highlighted session, or "" when the list is empty.
func (m *Model) Choice() string {
	if m.index(m.selected) < 0 {
		return ""
	}
	return m.selected
}

// Key handles one key press: arrows or j and k move, Enter chooses, Escape
// closes.
func (m *Model) Key(k tea.Key) Result {
	n := len(m.sessions)
	switch {
	case k.Code == tea.KeyEscape:
		return Closed
	case k.Code == tea.KeyEnter:
		if m.Choice() == "" {
			return Closed
		}
		return Chosen
	case n == 0:
	case k.Code == tea.KeyUp || k.Text == "k":
		m.selected = m.sessions[(max(m.index(m.selected), 0)-1+n)%n].Name
	case k.Code == tea.KeyDown || k.Text == "j":
		m.selected = m.sessions[(m.index(m.selected)+1)%n].Name
	}
	return Open
}

// Click handles a left click at screen cell x, y on a screen of cols by
// rows cells. A click on a session chooses it, a click elsewhere in the
// popup does nothing, and a click outside closes it.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return Closed
	}
	if i := y - l.y - 1; x > l.x && x < l.x+l.w-1 && i >= 0 && i < l.count {
		m.selected = m.sessions[l.first+i].Name
		return Chosen
	}
	return Open
}

// View draws the popup centered on a screen of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the screen is too small; the keys still answer.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := m.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, l.x, l.y
}

// layout is where the popup goes and which sessions it shows.
type layout struct {
	box          string
	x, y, w, h   int
	first, count int // the first session shown, and how many are shown
}

// chromeRows are the popup's rows besides the sessions: two frame rows, a
// blank, and the key hints.
const chromeRows = 4

// maxWidth keeps the popup readable on a wide screen.
const maxWidth = 64

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	w := min(maxWidth, cols-4)
	if w < 20 || rows < chromeRows+1 {
		return layout{}, false
	}
	l := layout{w: w, count: min(len(m.sessions), rows-chromeRows)}
	inner := w - 2
	var body []string
	if l.count == 0 {
		body = append(body, " "+lipgloss.NewStyle().Foreground(th.Muted).Render("No sessions."))
	} else {
		sel := max(m.index(m.selected), 0)
		l.first = min(max(sel-l.count+1, 0), len(m.sessions)-l.count)
		shown := m.sessions[l.first : l.first+l.count]
		items := Items(shown, m.now())
		for i, s := range shown {
			if s.Name == m.current {
				items[i].Cols = append(items[i].Cols, "current")
			}
		}
		list := ui.List(th, ui.ListOptions{Items: items, Selected: m.index(m.selected) - l.first, Width: inner})
		body = strings.Split(list, "\n")
	}
	hints := []ui.Hint{{Key: "enter", Desc: "switch"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "close"}}
	body = append(body, "", " "+ui.KeyHints(th, hints, inner-1))
	l.box = ui.Panel(th, ui.PanelOptions{Title: "Sessions", Body: strings.Join(body, "\n"), Width: w})
	l.w, l.h = ui.Size(l.box)
	l.x, l.y = max((cols-l.w)/2, 0), max((rows-l.h)/2, 0)
	return l, true
}
