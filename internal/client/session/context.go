package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/contextpane"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// contextPane fits a context pane to ownPane.
type contextPane struct {
	*contextpane.Model
	rev *uint64 // the newest state revision shown
}

func newContextPane(id int, th theme.Theme) contextPane {
	return contextPane{Model: contextpane.New(id, th), rev: new(uint64)}
}

// Feed shows a newer state of the pane. A state this client cannot read
// shows as the pane's error, so the user sees why nothing changes.
func (c contextPane) Feed(msg api.Message) {
	s, ok := msg.(*api.SidePaneState)
	if !ok || s.Rev < *c.rev {
		return
	}
	*c.rev = s.Rev
	var st api.ContextState
	if err := api.DecodeSideState(s, &st); err != nil {
		st = api.ContextState{Error: "This Snow cannot read the context pane's state: " + err.Error()}
	}
	c.SetState(st)
}

func (c contextPane) Key(k tea.Key) (api.Message, string) { return c.Model.Key(k), c.Note() }

func (c contextPane) Click(x, y int) (api.Message, string) { return c.Model.Click(x, y), c.Note() }

func (c contextPane) Animating() bool { return false }

func (c contextPane) Tick() {}
