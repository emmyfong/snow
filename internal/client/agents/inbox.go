package agents

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Inbox is the needs-you inbox: every agent of every session that waits for
// the user, oldest first, in a list popup. Enter jumps to the agent's pane;
// the user answers there.
type Inbox struct {
	pick   *listpick.Model
	agents []api.AgentInfo // the needs-you agents, oldest first
	now    func() time.Time
}

// NewInbox returns an empty inbox. Usage is not part of it: an agent that
// waits for the user matters more than its cost.
func NewInbox(now func() time.Time) *Inbox {
	hints := []ui.Hint{{Key: "enter", Desc: "go to pane"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "close"}}
	return &Inbox{pick: listpick.New("Needs you", hints), now: now}
}

// SetAgents keeps the needs-you agents of all, the server's whole list,
// oldest first. The selection stays on the same agent; when that one is
// answered or gone, it moves to the oldest agent left.
func (i *Inbox) SetAgents(all []api.AgentInfo) {
	was, had := i.selectedPane()
	i.agents = i.agents[:0]
	for _, a := range all {
		if a.State == api.AgentNeedsYou {
			i.agents = append(i.agents, a)
		}
	}
	// The pane breaks a tie so the order never changes between two lists
	// with the same times.
	slices.SortStableFunc(i.agents, func(a, b api.AgentInfo) int {
		if c := a.Since.Compare(b.Since); c != 0 {
			return c
		}
		return a.Pane - b.Pane
	})
	rows := make([][]string, len(i.agents))
	for n, a := range i.agents {
		rows[n] = []string{a.Session, place(a), kindName(a.Kind), waitText(a), span(i.now().Sub(a.Since))}
	}
	i.pick.SetRows(rows)
	i.pick.SetTitle(count(len(i.agents), "agent needs you", "agents need you"))
	i.pick.SetNote("No agent needs you.")
	if !had {
		return
	}
	if n := slices.IndexFunc(i.agents, func(a api.AgentInfo) bool { return a.Pane == was }); n >= 0 {
		i.pick.Select(n)
		return
	}
	i.pick.Select(0)
}

// selectedPane returns the selected agent's pane, and false when none is
// selected.
func (i *Inbox) selectedPane() (int, bool) {
	n := i.pick.Selected()
	if n < 0 || n >= len(i.agents) {
		return 0, false
	}
	return i.agents[n].Pane, true
}

// Choice returns the selected agent, and false when the inbox lists none.
func (i *Inbox) Choice() (api.AgentInfo, bool) {
	n := i.pick.Selected()
	if n < 0 || n >= len(i.agents) {
		return api.AgentInfo{}, false
	}
	return i.agents[n], true
}

// Key handles one key press. Enter accepts the selected agent; Esc closes.
func (i *Inbox) Key(k tea.Key) picker.Result { return i.pick.Key(k) }

// Click handles a left click at window cell x, y. A click on an agent's row
// accepts it.
func (i *Inbox) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	return i.pick.Click(th, cols, rows, x, y)
}

// View draws the inbox centered on a window of cols by rows cells.
func (i *Inbox) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	return i.pick.View(th, cols, rows)
}
