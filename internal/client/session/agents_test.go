package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// boardAgents lists two agents of session api, in windows 1 and 2, and
// one of session web.
func boardAgents(reviewer string) *api.AgentList {
	now := fixedClock()
	return &api.AgentList{Agents: []api.AgentInfo{
		{Session: "api", Window: 1, WindowName: "dev", Pane: 2, Kind: api.AgentKindClaudeCode, State: api.AgentWorking, Since: now},
		{Session: "api", Window: 2, WindowName: "review", Pane: 7, Kind: api.AgentKindClaudeCode, State: reviewer, Since: now},
		{Session: "web", Window: 0, WindowName: "elsewhere", Pane: 9, Kind: api.AgentKindChat, State: api.AgentIdle, Since: now},
	}}
}

// openBoard presses Ctrl+b w.
func openBoard(m *Model) tea.Cmd {
	m.Update(ctrlB)
	return m.Update(press('w', 0, "w"))
}

// TestAgentsBoardOpens: Ctrl+b w shows this session's agents over the
// window, and Esc closes the board.
func TestAgentsBoardOpens(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(boardAgents(api.AgentIdle))
	openBoard(m)
	view := plain(m)
	if !strings.Contains(view, "Agents in api") || !strings.Contains(view, "2 review") || strings.Contains(view, "elsewhere") {
		t.Fatalf("board:\n%s", view)
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	if strings.Contains(plain(m), "Agents in api") || len(r.sent) != 0 {
		t.Fatalf("Esc left the board open or sent %v", r.sent)
	}
}

// TestAgentsBoardJumpFocusesPane: Enter on a row asks the server to focus
// that agent's pane, which also shows its window.
func TestAgentsBoardJumpFocusesPane(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(boardAgents(api.AgentNeedsYou))
	openBoard(m)
	m.Update(enter)
	if len(r.sent) != 1 {
		t.Fatalf("sent %v, want one focus command", r.sent)
	}
	cmd, ok := r.sent[0].(*api.Command)
	if !ok || cmd.Op != api.OpFocus || cmd.Target != 7 || len(cmd.Args) != 0 {
		t.Fatalf("sent %#v, want focus of pane 7, the agent that needs you", r.sent[0])
	}
	if strings.Contains(plain(m), "Agents in api") {
		t.Fatal("the board stays open after Enter")
	}
}

// TestAgentsBoardFollowsTheServer: a new agent list changes the open
// board without a key press.
func TestAgentsBoardFollowsTheServer(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	m.now = fixedClock
	m.Update(boardAgents(api.AgentIdle))
	openBoard(m)
	if strings.Contains(plain(m), "needs you") {
		t.Fatalf("board before the change:\n%s", plain(m))
	}
	m.Update(boardAgents(api.AgentNeedsYou))
	if !strings.Contains(plain(m), "● needs you") {
		t.Fatalf("board after the change:\n%s", plain(m))
	}
}
