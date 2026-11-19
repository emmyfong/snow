package session

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/menu"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func TestMenuButtonOpensMenu(t *testing.T) {
	m := attached(t, 80, 24)
	_, spans := ui.WindowBarRow(m.theme, m.windowBarOptions())
	for _, sp := range spans {
		if sp.Kind == ui.TopBarMenu {
			m.Update(tea.MouseClickMsg{X: sp.X, Y: 0, Button: tea.MouseLeft})
		}
	}
	if m.menu == nil {
		t.Fatal("menu button did not open menu")
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.menu != nil {
		t.Fatal("Esc did not close menu")
	}
}

func chooseWindowRow(m *Model, index int, mouse bool) tea.Cmd {
	m.openWindowMenu()
	if mouse {
		_, x, y := m.menu.View(m.theme, m.cols, m.windowRows())
		return m.Update(tea.MouseClickMsg{X: x + 1, Y: y + 1 + index + m.areaTop(), Button: tea.MouseLeft})
	}
	for range index {
		m.Update(press(tea.KeyDown, 0, ""))
	}
	return m.Update(enter)
}

func TestMenuKeyboardMouseParity(t *testing.T) {
	for i, it := range menu.LayoutItems(keys.Default(), true) {
		t.Run(it.Label, func(t *testing.T) {
			var states []string
			var sent [][]api.Message
			var results []any
			for _, byMouse := range []bool{false, true} {
				m, r := modelScreen(t, 120, 40, "anthropic/claude-sonnet-5-5")
				r.sent = nil
				cmd := chooseWindowRow(m, i, byMouse)
				states = append(states, plain(m))
				sent = append(sent, r.sent)
				var result any
				if cmd != nil {
					result = cmd()
				}
				results = append(results, result)
				if m.menu != nil {
					t.Fatal("chosen menu did not close")
				}
			}
			if states[0] != states[1] || !reflect.DeepEqual(sent[0], sent[1]) || !reflect.DeepEqual(results[0], results[1]) {
				t.Fatalf("keyboard and mouse differ: sends %v / %v; results %v / %v", sent[0], sent[1], results[0], results[1])
			}
		})
	}
}

func TestMenuCommandRow(t *testing.T) {
	m := attached(t, 80, 24)
	chooseWindowRow(m, 5, false)
	if m.prompt == nil || m.prompt.mode != modeThemes {
		t.Fatal("Theme did not run session theme command")
	}
}

func testSessionHelpRow(t *testing.T, row int) {
	t.Helper()
	m := attached(t, 80, 24)
	chooseWindowRow(m, row, false)
	if m.over == nil || !strings.Contains(plain(m), "Help") || !strings.Contains(plain(m), "/menu") {
		t.Fatalf("row %d did not open session help", row)
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.over != nil {
		t.Fatal("Esc did not close help")
	}
}

func TestHelpRowOpensPanelInSession(t *testing.T) { testSessionHelpRow(t, 6) }

func sessionHelpGolden(t *testing.T, cols, rows int) {
	t.Helper()
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	chooseWindowRow(m, 6, false)
	golden.RequireEqual(t, m.View().Content)
}

func TestSessionHelpGolden80x24(t *testing.T)  { sessionHelpGolden(t, 80, 24) }
func TestSessionHelpGolden120x40(t *testing.T) { sessionHelpGolden(t, 120, 40) }

func TestConnectRowOpensLoginPicker(t *testing.T) {
	m, r := modelScreen(t, 80, 24, "anthropic/claude-sonnet-5-5")
	chooseWindowRow(m, 5, false)
	if m.authPick == nil || m.authPane != 2 || !sentAuthListRequest(r) {
		t.Fatal("Connect did not open focused chat login picker")
	}
}

func TestMenuCommandOpens(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	typeIn(m, "menu\r")
	if m.menu == nil || m.prompt != nil {
		t.Fatal("menu command did not open window menu")
	}
}

func TestMenuRowShowsShortcut(t *testing.T) {
	m := attached(t, 80, 24)
	m.keys = rebound(t)
	m.openWindowMenu()
	view := plain(m)
	if shortcut := m.keys.Shortcut("split-right"); !strings.Contains(view, shortcut) {
		t.Fatalf("menu omits rebound shortcut %q", shortcut)
	}
}

func TestSessionHelpListsConfiguredKeys(t *testing.T) {
	m := screen(120, 40)
	m.keys = rebound(t)
	m.openSessionHelp()
	if !strings.Contains(plain(m), m.keys.PrefixLabel()) || !strings.Contains(plain(m), "|") {
		t.Fatal("help omits configured prefix or split key")
	}
	if _, ok := command.Find("menu", command.InSession); !ok {
		t.Fatal("menu command not registered")
	}
}

func TestLoginPickerExplainsTerminalLimitation(t *testing.T) {
	m := attached(t, 80, 24)
	m.openLoginPicker(1)
	if !strings.Contains(m.message, "chat panes") {
		t.Fatalf("terminal login picker did not explain its limit: %q", m.message)
	}
}
