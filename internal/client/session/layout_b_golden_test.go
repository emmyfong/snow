package session

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func layoutBGolden(t *testing.T, cols, rows int, setup func(*Model)) {
	t.Helper()
	m := screen(cols, rows)
	m.now = fixedClock
	panes := []api.PaneInfo{{ID: 1, Kind: api.PaneTerminal, Profile: "sh", Folder: "/home/a/Projects/snow", Focused: true, Rect: api.Rect{Y: 1, W: cols, H: rows - 3}}}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	if setup != nil {
		setup(m)
	}
	for _, w := range m.layout.Windows {
		fill(m, w.Panes)
	}
	golden.RequireEqual(t, m.View().Content)
}

func layoutBTwoPanes(m *Model) {
	left := m.cols / 2
	panes := []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Profile: "sh", Folder: "/home/a/Projects/snow", Focused: true, Rect: api.Rect{Y: 1, W: left, H: m.rows - 3}},
		{ID: 2, Kind: api.PaneTerminal, Profile: "logs", Folder: "/home/a/Projects/snow", Rect: api.Rect{X: left + 1, Y: 1, W: m.cols - left - 1, H: m.rows - 3}},
	}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
}

func layoutBTwoWindows(m *Model) {
	first := m.layout.Windows[0]
	second := api.WindowInfo{Index: 2, Name: "api", Panes: []api.PaneInfo{{ID: 2, Kind: api.PaneTerminal, Profile: "sh", Folder: "/home/a/Projects/snow", Focused: true, Rect: api.Rect{Y: 1, W: m.cols, H: m.rows - 3}}}}
	m.Update(&api.Layout{Session: "work", Active: 2, Windows: []api.WindowInfo{first, second}})
}

func layoutBNeedsYou(m *Model) {
	layoutBTwoWindows(m)
	m.layout.Windows[0].AgentState = api.AgentNeedsYou
	m.agents = []api.AgentInfo{{Session: "work", Window: 1, Pane: 1, State: api.AgentNeedsYou}}
}

func layoutBUnfocused(m *Model) {
	layoutBTwoPanes(m)
	m.layout.Windows[0].Panes[0].Focused = false
	m.layout.Windows[0].Panes[1].Focused = true
}

func TestLayoutBIdleGolden80x24(t *testing.T)         { layoutBGolden(t, 80, 24, nil) }
func TestLayoutBIdleGolden120x40(t *testing.T)        { layoutBGolden(t, 120, 40, nil) }
func TestLayoutBOneWindowGolden80x24(t *testing.T)    { layoutBGolden(t, 80, 24, layoutBTwoPanes) }
func TestLayoutBOneWindowGolden120x40(t *testing.T)   { layoutBGolden(t, 120, 40, layoutBTwoPanes) }
func TestLayoutBTwoWindowsGolden80x24(t *testing.T)   { layoutBGolden(t, 80, 24, layoutBTwoWindows) }
func TestLayoutBTwoWindowsGolden120x40(t *testing.T)  { layoutBGolden(t, 120, 40, layoutBTwoWindows) }
func TestLayoutBNeedsYouTabGolden80x24(t *testing.T)  { layoutBGolden(t, 80, 24, layoutBNeedsYou) }
func TestLayoutBNeedsYouTabGolden120x40(t *testing.T) { layoutBGolden(t, 120, 40, layoutBNeedsYou) }
func TestLayoutBMenuOpenGolden80x24(t *testing.T)     { layoutBGolden(t, 80, 24, (*Model).openWindowMenu) }
func TestLayoutBMenuOpenGolden120x40(t *testing.T) {
	layoutBGolden(t, 120, 40, (*Model).openWindowMenu)
}
func TestLayoutBFocusedPaneGolden80x24(t *testing.T)    { layoutBGolden(t, 80, 24, layoutBTwoPanes) }
func TestLayoutBFocusedPaneGolden120x40(t *testing.T)   { layoutBGolden(t, 120, 40, layoutBTwoPanes) }
func TestLayoutBUnfocusedPaneGolden80x24(t *testing.T)  { layoutBGolden(t, 80, 24, layoutBUnfocused) }
func TestLayoutBUnfocusedPaneGolden120x40(t *testing.T) { layoutBGolden(t, 120, 40, layoutBUnfocused) }
