package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/files"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// filesPane fits a files pane to ownPane.
type filesPane struct {
	*files.Model
	rev *uint64 // the newest state revision shown
}

func newFilesPane(id int, th theme.Theme) filesPane {
	return filesPane{Model: files.New(id, th), rev: new(uint64)}
}

// Feed shows a newer state of the pane. A state this client cannot read
// shows as the pane's error, so the user sees why nothing changes.
func (f filesPane) Feed(msg api.Message) {
	s, ok := msg.(*api.SidePaneState)
	if !ok || s.Rev < *f.rev {
		return
	}
	*f.rev = s.Rev
	var st api.FilesState
	if err := api.DecodeSideState(s, &st); err != nil {
		st = api.FilesState{Error: "This Snow cannot read the files pane's state: " + err.Error()}
	}
	f.SetState(st)
}

func (f filesPane) Key(k tea.Key) (api.Message, string) { return f.Model.Key(k), f.Note() }

func (f filesPane) Click(x, y int) (api.Message, string) { return f.Model.Click(x, y), f.Note() }

func (f filesPane) Animating() bool { return false }

func (f filesPane) Tick() {}
