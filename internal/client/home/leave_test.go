package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func TestCtrlCAsksBeforeLeaving(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	if got := msgOf(m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))); got != nil || m.confirm == nil {
		t.Fatalf("Ctrl+c gave %#v, popup open %v; want a leave popup", got, m.confirm != nil)
	}
}

func TestLeaveKeysAndMouse(t *testing.T) {
	tests := []struct {
		name string
		act  func(*Model) tea.Msg
		want bool
	}{
		{name: "y", act: func(m *Model) tea.Msg { return msgOf(m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))) }, want: true},
		{name: "n", act: func(m *Model) tea.Msg { return msgOf(m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))) }},
		{name: "escape", act: func(m *Model) tea.Msg { return msgOf(m.Update(press(tea.KeyEscape))) }},
		{name: "enter defaults to cancel", act: func(m *Model) tea.Msg { return msgOf(m.Update(press(tea.KeyEnter))) }},
		{name: "click Leave", act: func(m *Model) tea.Msg { return clickHomeLabel(t, m, "[ Leave ]") }, want: true},
		{name: "click Cancel", act: func(m *Model) tea.Msg { return clickHomeLabel(t, m, "[ Cancel ]") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := screen(80, 24, session("api", time.Minute, 0))
			m.askLeave()
			got := tt.act(m)
			if (got == QuitMsg{}) != tt.want || m.confirm != nil {
				t.Fatalf("got %#v, popup open %v; want quit %v and closed popup", got, m.confirm != nil, tt.want)
			}
		})
	}
}

func TestLeaveTextCountsSessions(t *testing.T) {
	tests := []struct {
		name     string
		sessions []api.SessionInfo
		want     string
	}{
		{name: "none", want: "Leave Snow?"},
		{name: "one", sessions: []api.SessionInfo{{Name: "api", State: api.StateRunning}}, want: "Leave Snow? Your 1 session keeps running."},
		{name: "two", sessions: []api.SessionInfo{{Name: "api", State: api.StateRunning}, {Name: "web", State: api.StateRunning}}, want: "Leave Snow? Your 2 sessions keep running."},
		{name: "ignore saved", sessions: []api.SessionInfo{{Name: "api", State: api.StateRunning}, {Name: "old", State: api.StateSaved}}, want: "Leave Snow? Your 1 session keeps running."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := screen(80, 24, tt.sessions...)
			m.askLeave()
			if got := ansi.Strip(m.View().Content); !strings.Contains(got, tt.want) {
				t.Fatalf("popup does not contain %q:\n%s", tt.want, got)
			}
		})
	}
}

func TestLeaveConfirmGolden80x24(t *testing.T)  { leaveConfirmGolden(t, 80, 24) }
func TestLeaveConfirmGolden120x40(t *testing.T) { leaveConfirmGolden(t, 120, 40) }

func leaveConfirmGolden(t *testing.T, cols, rows int) {
	m := screen(cols, rows, session("api-server", time.Minute, 1))
	m.askLeave()
	golden.RequireEqual(t, m.View().Content)
}

func clickHomeLabel(t *testing.T, m *Model, label string) tea.Msg {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			return msgOf(m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}))
		}
	}
	t.Fatalf("%q not found:\n%s", label, ansi.Strip(m.View().Content))
	return nil
}
