package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// inboxAgents is an agent list with one agent of this session waiting and
// one of session web.
func inboxAgents() *api.AgentList {
	now := fixedClock()
	return &api.AgentList{Agents: []api.AgentInfo{
		{Session: "api", Window: 1, WindowName: "dev", Pane: 2, Kind: api.AgentKindChat, State: api.AgentNeedsYou, Since: now},
		{Session: "web", Window: 0, WindowName: "elsewhere", Pane: 9, Kind: api.AgentKindClaudeCode, State: api.AgentNeedsYou, Since: now},
	}}
}

// openInbox presses Ctrl+b i.
func openInbox(m *Model) tea.Cmd {
	m.Update(ctrlB)
	return m.Update(press('i', 0, "i"))
}

// TestInboxKeyOpens: Ctrl+b i shows the needs-you inbox over the window.
func TestInboxKeyOpens(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(inboxAgents())
	openInbox(m)
	if view := plain(m); !strings.Contains(view, "2 agents need you") || !strings.Contains(view, "api") {
		t.Fatalf("inbox not drawn:\n%s", view)
	}
}

// TestInboxJump: Enter on an agent of this session asks the server to focus
// its pane.
func TestInboxJump(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(inboxAgents())
	openInbox(m)
	m.Update(enter)
	c := lastCommand(t, r)
	if c.Op != api.OpFocus || c.Target != 2 {
		t.Fatalf("sent %+v, want focus of pane 2", c)
	}
	if strings.Contains(plain(m), "agents need you") {
		t.Fatal("the inbox stays open after Enter")
	}
}

// TestInboxJumpsToAnotherSession: Enter on an agent of another session asks
// the client to switch to that session and focus the pane.
func TestInboxJumpsToAnotherSession(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(&api.AgentList{Agents: inboxAgents().Agents[1:]})
	openInbox(m)
	s, ok := switchOf(m.Update(enter))
	if !ok || s != (SwitchMsg{Session: "web", Pane: 9}) {
		t.Fatalf("Enter gave %+v %v, want a switch to web at pane 9", s, ok)
	}
}

// TestInboxFollowsTheServer: a new agent list changes the open inbox
// without a key press.
func TestInboxFollowsTheServer(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(inboxAgents())
	openInbox(m)
	m.Update(&api.AgentList{Agents: []api.AgentInfo{
		{Session: "api", Window: 2, WindowName: "review", Pane: 6, Kind: api.AgentKindClaudeCode, State: api.AgentNeedsYou, Since: fixedClock()},
	}})
	view := plain(m)
	if !strings.Contains(view, "1 agent needs you") || !strings.Contains(view, "review") {
		t.Fatalf("inbox after the change:\n%s", view)
	}
}
