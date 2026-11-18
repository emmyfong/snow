package session

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func TestThemeSessionGolden80x24(t *testing.T)  { themeSessionGolden(t, 80, 24) }
func TestThemeSessionGolden120x40(t *testing.T) { themeSessionGolden(t, 120, 40) }

func themeSessionGolden(t *testing.T, cols, rows int) {
	t.Helper()
	for _, th := range theme.Builtin() {
		t.Run(th.Name, func(t *testing.T) {
			m := screen(cols, rows)
			m.SetTheme(th)
			panes := []api.PaneInfo{{
				ID: 1, Kind: api.PaneTerminal, Profile: "shell", Folder: "/work/snow", Focused: true,
				Rect: api.Rect{Y: 1, W: cols, H: rows - 3},
			}}
			m.Update(&api.Layout{Session: "work", Active: 2, Windows: []api.WindowInfo{
				{Index: 1, Name: "dev", AgentState: api.AgentNeedsYou},
				{Index: 2, Name: "logs", Panes: panes},
			}})
			m.Update(&api.AgentList{Agents: []api.AgentInfo{{State: api.AgentNeedsYou}}})
			fill(m, panes)
			golden.RequireEqual(t, m.View().Content)
		})
	}
}
