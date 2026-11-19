package settings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
)

type testGroup struct {
	updates int
	leaves  int
}

func (g *testGroup) Title() string { return "Test group" }

func (g *testGroup) Update(tea.Msg) tea.Cmd {
	g.updates++
	return nil
}

func (g *testGroup) Leave() tea.Cmd {
	g.leaves++
	return nil
}

func (*testGroup) View(theme.Theme, int, int) string { return "Group contents" }

func TestGroupEscLeavesGroupFirst(t *testing.T) {
	g := &testGroup{}
	m := New([]Group{g})
	m.SetSize(80, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.active {
		t.Fatal("Enter did not focus the group")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.active || g.leaves != 1 || g.updates != 1 {
		t.Fatalf("after Esc active=%v leaves=%d updates=%d, want list focus and one Leave", m.active, g.leaves, g.updates)
	}
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc on the group list returned no CloseMsg command")
	}
	if _, ok := cmd().(CloseMsg); !ok {
		t.Fatalf("Esc on the group list returned %T, want CloseMsg", cmd())
	}
}

func TestSettingsEmptyGolden80x24(t *testing.T) {
	settingsGolden(t, 80, 24)
}

func TestSettingsEmptyGolden120x40(t *testing.T) {
	settingsGolden(t, 120, 40)
}

func settingsGolden(t *testing.T, cols, rows int) {
	t.Helper()
	m := New(nil)
	m.SetTheme(theme.Default())
	m.SetSize(cols, rows)
	golden.RequireEqual(t, m.View().Content)
}
