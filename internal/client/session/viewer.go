package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/viewer"
	"github.com/emmyfong/snow/pkg/api"
)

// viewerPane fits a viewer pane to ownPane.
type viewerPane struct {
	*viewer.Model
	rev *uint64 // the newest state revision shown
}

func newViewerPane(id int, th theme.Theme) viewerPane {
	return viewerPane{Model: viewer.New(id, th), rev: new(uint64)}
}

// Feed shows a newer state of the pane. A state this client cannot read
// shows as the pane's error, so the user sees why nothing changes.
func (v viewerPane) Feed(msg api.Message) {
	s, ok := msg.(*api.SidePaneState)
	if !ok || s.Rev < *v.rev {
		return
	}
	*v.rev = s.Rev
	var st api.ViewerState
	if err := api.DecodeSideState(s, &st); err != nil {
		st = api.ViewerState{Error: "This Snow cannot read the viewer's state: " + err.Error()}
	}
	v.SetState(st)
}

func (v viewerPane) Key(k tea.Key) (api.Message, string) { return v.Model.Key(k), v.Note() }

func (v viewerPane) Click(x, y int) (api.Message, string) { return v.Model.Click(x, y), v.Note() }

func (v viewerPane) Animating() bool { return false }

func (v viewerPane) Tick() {}
