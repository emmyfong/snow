package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/layoutpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// arrange asks the server to arrange the window of pane id as preset name.
func (m *Model) arrange(id int, name string) {
	m.send(&api.Command{Op: api.OpLayout, Target: id, Args: map[string]string{api.ArgName: strings.ToLower(name)}})
}

// openLayoutPicker shows the layout presets, for the window of pane id.
func (m *Model) openLayoutPicker(id int) tea.Cmd {
	pick := layoutpick.New()
	return m.showOverlay(&overlay{
		pane:  id,
		popup: pick,
		key:   pick.Key,
		click: pick.Click,
		done: func(r picker.Result) tea.Cmd {
			if name := pick.Choice(); name != "" && r == picker.Accepted {
				m.arrange(id, name)
			}
			return nil
		},
	}, "the layout picker")
}

// askedArg returns what own pane p's last ask named, or "".
func askedArg(p ownPane) string {
	if a, ok := p.(interface{ AskedArg() string }); ok {
		return a.AskedArg()
	}
	return ""
}
