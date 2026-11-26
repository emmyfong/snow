package session

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func TestGitMenuItems(t *testing.T) {
	for _, tt := range []struct{ label, shortcut, action string }{
		{"Stage", "s", api.GitActStage}, {"Diff", "Enter", api.GitActDiff}, {"Stage all", "a", api.GitActStageAll}, {"Refresh", "r", api.GitActRefresh}, {"Commit...", "c", ""},
	} {
		m, r := gitScreen(t, 80, 24)
		m.openMenu(41, 2)
		if m.menu == nil {
			t.Fatal("git menu does not fit 80x24")
		}
		view := ansi.Strip(content(m))
		found := false
		for _, row := range strings.Split(view, "\n") {
			if strings.Contains(row, tt.label) && strings.Contains(row, tt.shortcut) {
				found = true
			}
		}
		if !found {
			t.Fatalf("menu lacks %s with %s:\n%s", tt.label, tt.shortcut, view)
		}
		for i := 0; i < 40; i++ {
			box, x, y := m.menu.View(m.theme, m.cols, m.windowRows())
			for row, text := range strings.Split(ansi.Strip(box), "\n") {
				if strings.Contains(text, tt.label) && !(tt.label == "Stage" && (strings.Contains(text, "Stage all") || strings.Contains(text, "Stage or unstage"))) {
					m.menuResult(m.menu.Click(m.theme, m.cols, m.windowRows(), x+2, y+row))
					goto chosen
				}
			}
			m.menu.Key(tea.Key{Code: tea.KeyDown})
		}
		t.Fatal("menu action not reachable")
	chosen:
		if tt.action != "" {
			if len(r.sent) == 0 {
				t.Fatalf("%s sent nothing", tt.label)
			}
			got, ok := r.sent[len(r.sent)-1].(*api.SidePaneAction)
			if !ok || got.Pane != 2 || got.Action != tt.action {
				t.Fatalf("%s sent %#v", tt.label, r.sent[len(r.sent)-1])
			}
		} else if _, _, ok := m.own[2].Cursor(); !ok {
			t.Fatal("Commit menu did not open message box")
		}
	}
}

func TestGitPaneFocusButtons(t *testing.T) {
	m, _ := gitScreen(t, 80, 24)
	git := m.own[2].(gitPane)
	if !strings.Contains(ansi.Strip(strings.Join(git.View(), "\n")), "Commit...") {
		t.Fatal("focused git pane has no buttons")
	}
	m.layout.Windows[0].Panes[1].Focused = false
	m.layout.Windows[0].Panes[0].Focused = true
	m.syncPanes()
	if strings.Contains(ansi.Strip(strings.Join(git.View(), "\n")), "Commit...") {
		t.Fatal("unfocused git pane keeps buttons")
	}
}

func TestGitMenuGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 8}} {
		for _, end := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d_end_%v", size[0], size[1], end), func(t *testing.T) {
				m, _ := gitScreen(t, size[0], size[1])
				m.openMenu(size[0]/2, 0)
				if m.menu == nil {
					t.Fatal("pane menu did not open")
				}
				if end {
					m.menu.Key(tea.Key{Code: tea.KeyUp})
				}
				golden.RequireEqual(t, content(m))
			})
		}
	}
}

func TestGitMenuWheel(t *testing.T) {
	m, _ := gitScreen(t, 80, 24)
	m.openMenu(41, 0)
	if m.menu == nil {
		t.Fatal("no menu")
	}
	m.menuMouse(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelDown})
	if m.menu.Selected() != wheelRows {
		t.Fatalf("wheel selected %d", m.menu.Selected())
	}
	m.menuMouse(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelUp})
	if m.menu.Selected() != 0 {
		t.Fatal("wheel did not move back")
	}
}
