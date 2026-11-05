package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/gitpane"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// gitPane fits a git pane to ownPane.
type gitPane struct {
	*gitpane.Model
	rev *uint64 // the newest state revision shown
}

func newGitPane(id int, th theme.Theme) gitPane {
	return gitPane{Model: gitpane.New(id, th), rev: new(uint64)}
}

// Feed shows a newer state of the pane. A state this client cannot read
// shows as the pane's error, so the user sees why nothing changes.
func (g gitPane) Feed(msg api.Message) {
	s, ok := msg.(*api.SidePaneState)
	if !ok || s.Rev < *g.rev {
		return
	}
	*g.rev = s.Rev
	var st api.GitState
	if err := api.DecodeSideState(s, &st); err != nil {
		st = api.GitState{Error: "This Snow cannot read the git pane's state: " + err.Error()}
	}
	g.SetState(st)
}

func (g gitPane) Key(k tea.Key) (api.Message, string) { return g.result(g.Model.Key(k)) }

func (g gitPane) Click(x, y int) (api.Message, string) { return g.result(g.Model.Click(x, y)) }

func (g gitPane) Animating() bool { return false }

func (g gitPane) Tick() {}

// result turns the pane's action into a message, never a typed nil, and
// the pane's note.
func (g gitPane) result(a *api.SidePaneAction) (api.Message, string) {
	note := g.Note()
	if a == nil {
		return nil, note
	}
	return a, note
}
