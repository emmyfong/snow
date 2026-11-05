package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// agentScreen is pickerScreen with two agent profiles. With worktree set,
// the focused pane is an agent on branch agent/1 in a worktree.
func agentScreen(t *testing.T, cols, rows int, worktree bool) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := screen(cols, rows)
	m.send = r.send
	m.Update(&api.ProfileList{Profiles: []string{"PowerShell", "Claude Code", "Ubuntu Claude Code"}, Default: "PowerShell", Agents: []string{"Claude Code", "Ubuntu Claude Code"}})
	panes := threePanes(cols, rows)
	panes[1].Folder = "/src"
	if worktree {
		panes[1].Worktree, panes[1].Branch = "/src/.worktrees/agent-1", "agent/1"
	}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	return m, r
}

// TestAgentLaunchByKey: Ctrl+b A opens the form on the focused pane's
// folder; typing a branch and Enter sends agent-new from that pane.
func TestAgentLaunchByKey(t *testing.T) {
	m, r := agentScreen(t, 80, 24, false)
	m.Update(ctrlB)
	m.Update(press('A', 0, "A"))
	if m.over == nil || len(r.sent) != 0 {
		t.Fatalf("form open %v, sent %v; want the form first", m.over != nil, r.sent)
	}
	m.Update(tea.PasteMsg{Content: "fix-login"})
	m.Update(enter)
	c := lastCommand(t, r)
	if c.Op != api.OpAgentNew || c.Target != 2 || c.Args[api.ArgFolder] != "/src" ||
		c.Args[api.ArgBranch] != "fix-login" || c.Args[api.ArgProfile] != "Claude Code" {
		t.Fatalf("sent %+v", c)
	}
}

// TestAgentLaunchFromMenuAndCommand: the pane menu and "agents new" open
// the same form; "agents" alone opens the board.
func TestAgentLaunchFromMenuAndCommand(t *testing.T) {
	m, _ := agentScreen(t, 80, 24, false)
	chooseByClick(t, m, "New agent in worktree...")
	if m.over == nil {
		t.Fatal("the menu did not open the form")
	}
	m, _ = agentScreen(t, 80, 24, false)
	m.Update(ctrlB)
	m.Update(press(':', 0, ":"))
	typeIn(m, "agents new")
	m.Update(enter)
	if m.over == nil {
		t.Fatal("agents new did not open the form")
	}
}

// TestCloseAgentAsksAboutWorktree: closing an agent with a worktree offers
// to keep or remove it; r removes, y keeps, and the branch is never named
// for removal.
func TestCloseAgentAsksAboutWorktree(t *testing.T) {
	for _, tt := range []struct {
		key    tea.KeyPressMsg
		remove bool
	}{
		{press('y', 0, "y"), false},
		{press('r', 0, "r"), true},
	} {
		m, r := agentScreen(t, 80, 24, true)
		m.Update(ctrlB)
		m.Update(press('x', 0, "x"))
		m.Update(tt.key)
		c := lastCommand(t, r)
		if c.Op != api.OpClose || c.Target != 2 || (c.Args[api.ArgWorktree] == api.WorktreeRemove) != tt.remove {
			t.Fatalf("%s sent %+v, want remove=%v", tt.key.Text, c, tt.remove)
		}
	}
	m, r := agentScreen(t, 80, 24, true)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	clickLabel(t, m, "[ Remove worktree ]", leftClick)
	if c := lastCommand(t, r); c.Args[api.ArgWorktree] != api.WorktreeRemove {
		t.Fatalf("click sent %+v, want the worktree removed", c)
	}
}

func agentCloseGolden(t *testing.T, cols, rows int) {
	m, _ := agentScreen(t, cols, rows, true)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	golden.RequireEqual(t, m.View().Content)
}

func TestCloseAgentGolden80x24(t *testing.T)  { agentCloseGolden(t, 80, 24) }
func TestCloseAgentGolden120x40(t *testing.T) { agentCloseGolden(t, 120, 40) }
