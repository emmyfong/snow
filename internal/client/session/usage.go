package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/usage"
	"github.com/emmyfong/snow/pkg/api"
)

// usagePane fits a usage pane to ownPane.
type usagePane struct {
	*usage.Model
	rev *uint64 // the newest state revision shown
}

func newUsagePane(id int, th theme.Theme) usagePane {
	return usagePane{Model: usage.New(id, th), rev: new(uint64)}
}

// Feed shows a newer state of the pane. A state this client cannot read
// shows as the pane's error, so the user sees why nothing changes.
func (u usagePane) Feed(msg api.Message) {
	s, ok := msg.(*api.SidePaneState)
	if !ok || s.Rev < *u.rev {
		return
	}
	*u.rev = s.Rev
	var st api.UsageState
	if err := api.DecodeSideState(s, &st); err != nil {
		st = api.UsageState{Error: "This Snow cannot read the usage pane's state: " + err.Error()}
	}
	u.SetState(st)
}

func (u usagePane) Key(k tea.Key) (api.Message, string) { return u.Model.Key(k), u.Note() }

func (u usagePane) Click(x, y int) (api.Message, string) { return u.Model.Click(x, y), u.Note() }

func (u usagePane) Animating() bool { return false }

func (u usagePane) Tick() {}
