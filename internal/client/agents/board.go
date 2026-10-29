// Package agents shows the server's agent list (api.AgentList): the agents
// board, which lists every agent of the session, and the needs-you inbox,
// which lists every agent that waits for the user. Both jump to the
// agent's pane; the user answers there.
package agents

import (
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// page is how many rows PgUp and PgDn move.
const page = 5

// boardChrome is the rows the board spends beside the list: two frame
// edges, the summary row, two blank rows, and the key hints.
const boardChrome = 6

// Smallest window the board draws in.
const minBoardWidth, minBoardHeight = 30, boardChrome + 1

// Board is the agents board: every agent of one session, one row each,
// over the whole window area. It is a popup: the session screen gives it
// keys and clicks until it closes.
type Board struct {
	session string
	agents  []api.AgentInfo // the session's, by window and pane
	pane    int             // the selected agent's pane, or 0
	usage   bool            // show the est. cost of chat agents
	now     func() time.Time
}

// NewBoard returns the board of session with no agents yet. Usage shows
// the estimated cost of chat agents, as their top bar does.
func NewBoard(session string, usage bool, now func() time.Time) *Board {
	return &Board{session: session, usage: usage, now: now}
}

// SetAgents shows the agents of the board's session in all, the server's
// whole list. The selection stays on the same agent; when that one is
// gone, it moves to the first agent that needs the user, else the first.
func (b *Board) SetAgents(all []api.AgentInfo) {
	b.agents = b.agents[:0]
	for _, a := range all {
		if a.Session == b.session {
			b.agents = append(b.agents, a)
		}
	}
	if b.index() >= 0 {
		return
	}
	b.pane = 0
	if i := slices.IndexFunc(b.agents, func(a api.AgentInfo) bool { return a.State == api.AgentNeedsYou }); i >= 0 {
		b.pane = b.agents[i].Pane
	} else if len(b.agents) > 0 {
		b.pane = b.agents[0].Pane
	}
}

// index is the selected agent's row, or -1.
func (b *Board) index() int {
	return slices.IndexFunc(b.agents, func(a api.AgentInfo) bool { return a.Pane == b.pane })
}

// Choice returns the selected agent's pane, and false when no agent is
// listed.
func (b *Board) Choice() (int, bool) { return b.pane, b.index() >= 0 }

// Key handles one key press: arrows, j and k, PgUp, PgDn, Home, and End
// move; Enter accepts the selected agent; Esc and q close.
func (b *Board) Key(k tea.Key) picker.Result {
	n := len(b.agents)
	i := max(b.index(), 0)
	switch {
	case k.Code == tea.KeyEscape || k.Text == "q":
		return picker.Cancelled
	case k.Code == tea.KeyEnter:
		if n == 0 {
			return picker.Cancelled
		}
		return picker.Accepted
	case n == 0:
		return picker.Open
	case k.Code == tea.KeyUp || k.Text == "k":
		i = picker.Move(i, -1, n)
	case k.Code == tea.KeyDown || k.Text == "j":
		i = picker.Move(i, 1, n)
	case k.Code == tea.KeyPgUp:
		i = max(i-page, 0)
	case k.Code == tea.KeyPgDown:
		i = min(i+page, n-1)
	case k.Code == tea.KeyHome:
		i = 0
	case k.Code == tea.KeyEnd:
		i = n - 1
	}
	b.pane = b.agents[i].Pane
	return picker.Open
}

// boardLayout is the drawn board and where its rows are.
type boardLayout struct {
	box   string
	top   int   // the window row of the first list row
	index []int // the agent on each list row
}

func (b *Board) layout(th theme.Theme, cols, rows int) (boardLayout, bool) {
	if cols < minBoardWidth || rows < minBoardHeight {
		return boardLayout{}, false
	}
	inner := cols - 2
	n := rows - boardChrome
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	body := []string{" " + b.summary()}
	body = append(body, "")
	var index []int
	if len(b.agents) == 0 {
		body = append(body, " "+muted.Render("No agents in this session."))
	} else {
		var list string
		list, index = ui.ListRows(th, ui.ListOptions{Items: b.items(th.Glyphs), Selected: b.index(), Width: inner - 1, Height: n})
		for _, l := range strings.Split(list, "\n") {
			body = append(body, " "+l)
		}
	}
	for len(body) < n+2 {
		body = append(body, "")
	}
	hints := []ui.Hint{{Key: "enter", Desc: "go to pane"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "close"}}
	body = append(body, "", " "+ui.KeyHints(th, hints, inner-1))
	box := ui.Panel(th, ui.PanelOptions{Title: "Agents in " + b.session, Body: strings.Join(body, "\n"), Width: cols})
	return boardLayout{box: box, top: 3, index: index}, true
}

// summary counts the agents, those that need the user, and, with usage
// on, the session's estimated cost.
func (b *Board) summary() string {
	waiting, cost, priced := 0, 0.0, false
	for _, a := range b.agents {
		if a.State == api.AgentNeedsYou {
			waiting++
		}
		if a.Cost > 0 {
			cost, priced = cost+a.Cost, true
		}
	}
	out := count(len(b.agents), "agent", "agents")
	if waiting > 0 {
		out += ", " + count(waiting, "needs you", "need you")
	}
	if b.usage && priced {
		out += ", " + ui.Dollars(cost) + " est."
	}
	return out
}

// items is one list row per agent: its window, kind, state, how long it
// has been in that state, its folder, and for a chat its model and cost.
// A row whose agent needs the user ends in an alert that says what it
// waits for, in the prompt colors.
func (b *Board) items(g theme.Glyphs) []ui.ListItem {
	now := b.now()
	items := make([]ui.ListItem, len(b.agents))
	for i, a := range b.agents {
		cols := []string{place(a), kindName(a.Kind), stateText(a.State, g), span(now.Sub(a.Since)), ui.FolderName(a.Folder)}
		if a.Kind == api.AgentKindChat {
			model := a.Model
			if b.usage && a.Cost > 0 {
				model += "  " + ui.Dollars(a.Cost)
			}
			cols = append(cols, model)
		}
		items[i] = ui.ListItem{Cols: cols}
		if a.State == api.AgentNeedsYou {
			items[i].Alert = waitText(a)
		}
	}
	return items
}

// View draws the board over a window area of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the area is too small; the keys still answer.
func (b *Board) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := b.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, 0, 0
}

// Click handles a left click at window cell x, y. A click on an agent's
// row accepts it; any other click does nothing.
func (b *Board) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	l, ok := b.layout(th, cols, rows)
	if !ok {
		return picker.Cancelled
	}
	if r := y - l.top; x > 0 && x < cols-1 && r >= 0 && r < len(l.index) {
		b.pane = b.agents[l.index[r]].Pane
		return picker.Accepted
	}
	return picker.Open
}

// count is n with the singular or the plural word.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
