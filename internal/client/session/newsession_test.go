package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/newsession"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func TestOpenNewWindowDialog(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	if cmd := m.openNewWindow(); cmd != nil || m.over == nil {
		t.Fatalf("openNewWindow: overlay %v, command %v; want dialog", m.over != nil, cmd)
	}
	pick, ok := m.over.popup.(*newsession.Model)
	if !ok {
		t.Fatalf("overlay popup is %T, want new-session dialog", m.over.popup)
	}
	if got := pick.Choice().Folder; got != "/src" {
		t.Fatalf("dialog folder = %q, want focused pane folder /src", got)
	}
	for range 2 {
		m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	cmd := lastCommand(t, r)
	if m.over != nil || cmd.Op != api.OpNewWindow || cmd.Args[api.ArgFolder] != "/src" {
		t.Fatalf("accept: overlay %v, command %+v; want new window in /src", m.over != nil, cmd)
	}
}

func TestNewWindowLayout(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	for range 2 {
		m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if got := lastCommand(t, r); got.Op != api.OpNewWindow {
		t.Fatalf("accepted dialog sent %+v, want new window", got)
	}
	added := &api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Panes: []api.PaneInfo{{ID: 1}}},
		{Index: 2, Panes: []api.PaneInfo{{ID: 99}, {ID: 100}}},
	}}
	m.Update(added)
	got := lastCommand(t, r)
	if got.Op != api.OpLayout || got.Target != 99 || got.Args[api.ArgName] != "ide" {
		t.Fatalf("new layout command %+v, want layout ide for first pane 99", got)
	}
	count := len(r.sent)
	m.Update(added)
	if len(r.sent) != count {
		t.Fatalf("second layout sent more commands: %#v", r.sent[count:])
	}
}

func TestNewSessionWindowEscape(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.over != nil || len(r.sent) != 0 {
		t.Fatalf("Esc left overlay %v and sent %v; want closed without a command", m.over != nil, r.sent)
	}
}

func TestNewSessionClaudeOnePane(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingRuntime("ready-connection")
	m.Update(&api.ChatSnapshot{Pane: 99, Agent: "chat-generation", State: api.ChatStateIdle})
	var request *api.RuntimeRequest
	for _, message := range r.sent {
		if sent, ok := message.(*api.RuntimeRequest); ok {
			request = sent
		}
		if command, ok := message.(*api.Command); ok && (command.Op == api.OpSplit || command.Op == api.OpClose) {
			t.Fatalf("runtime startup changed panes with %+v", command)
		}
	}
	if request == nil || request.Pane != 99 || request.Agent != "chat-generation" || request.Action != api.RuntimeUse || request.Connection != "ready-connection" {
		t.Fatalf("runtime request %+v, want to bind the first chat to the selected connection", request)
	}
}

func TestNewSessionFromWindowBarGolden80x24(t *testing.T)  { newSessionFromWindowBarGolden(t, 80, 24) }
func TestNewSessionFromWindowBarGolden120x40(t *testing.T) { newSessionFromWindowBarGolden(t, 120, 40) }

func newSessionFromWindowBarGolden(t *testing.T, cols, rows int) {
	m, _ := pickerScreen(t, cols, rows)
	_, spans := ui.WindowBarRow(m.theme, m.windowBarOptions())
	for _, span := range spans {
		if span.Kind == ui.TopBarNewWindow {
			m.Update(click(span.X, 0))
			if m.over == nil {
				t.Fatal("the + control did not open the dialog")
			}
			golden.RequireEqual(t, m.View().Content)
			return
		}
	}
	t.Fatal("window bar has no + control")
}
