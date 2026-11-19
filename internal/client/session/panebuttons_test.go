package session

import (
	"testing"

	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func TestPaneButtonClick(t *testing.T) {
	for _, action := range []string{"model-picker", "zoom", "close-pane"} {
		t.Run(action, func(t *testing.T) {
			m, r := modelScreen(t, 80, 24, "anthropic/claude-sonnet-5-5")
			r.sent = nil
			buttons := ui.PaneButtons(m.windowPanes(), m.cols, m.windowRows())
			found := false
			for _, button := range buttons {
				if button.Action == action {
					found = true
					m.Update(click(button.X+button.W-1, button.Y+m.areaTop()))
					break
				}
			}
			if !found {
				t.Fatalf("missing %s button: %+v", action, buttons)
			}
			switch action {
			case "model-picker":
				if m.modelPick == nil || !sentListRequest(r) {
					t.Fatal("model button did not open picker")
				}
			case "zoom":
				if cmd := lastCommand(t, r); cmd.Op != api.OpZoom || cmd.Target != 2 {
					t.Fatalf("zoom sent %+v", cmd)
				}
			case "close-pane":
				if m.confirm == nil || m.drag != nil || len(r.sent) != 0 {
					t.Fatalf("close must ask without sending or dragging: %+v", r.sent)
				}
			}
		})
	}
}

func TestPaneNameUsesProfileThenKind(t *testing.T) {
	m := attached(t, 80, 24)
	panes := m.windowPanes()
	if panes[0].Name != "PowerShell" {
		t.Fatalf("profile name %q", panes[0].Name)
	}
	m.layout.Windows[0].Panes[0].Profile = ""
	m.layout.Windows[0].Panes[0].Kind = api.PaneTerminal
	if got := m.windowPanes()[0].Name; got != api.PaneTerminal {
		t.Fatalf("fallback name %q", got)
	}
}
