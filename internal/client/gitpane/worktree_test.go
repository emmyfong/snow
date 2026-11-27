package gitpane

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

// TestWorktreeOwner: an agent belongs to its worktree: the one Snow made
// for it, else the deepest one that holds its folder, whatever slashes
// and case a Windows path uses.
func TestWorktreeOwner(t *testing.T) {
	win := []api.GitWorktree{{Path: "C:/src/snow", Main: true}, {Path: "C:/src/snow/.worktrees/agent-1", Branch: "agent/1"}}
	wsl := []api.GitWorktree{{Path: "/home/u/snow", Main: true}, {Path: "/home/u/snow/.worktrees/agent-1", Branch: "agent/1"}}
	tests := []struct {
		name  string
		wts   []api.GitWorktree
		agent api.AgentInfo
		want  int // the worktree it belongs to, or -1
	}{
		{"windows backslashes and case", win, api.AgentInfo{Folder: `c:\Src\snow\.worktrees\agent-1`}, 1},
		{"windows subfolder", win, api.AgentInfo{Folder: `C:\src\snow\.worktrees\agent-1\internal`}, 1},
		{"main checkout", win, api.AgentInfo{Folder: `C:\src\snow\internal`}, 0},
		{"worktree Snow made", win, api.AgentInfo{Folder: `C:\other`, Worktree: `C:\src\snow\.worktrees\agent-1`}, 1},
		{"wsl", wsl, api.AgentInfo{Folder: "/home/u/snow/.worktrees/agent-1/"}, 1},
		{"wsl is case sensitive", wsl, api.AgentInfo{Folder: "/home/u/Snow"}, -1},
		{"a folder beside, not below", wsl, api.AgentInfo{Folder: "/home/u/snowball"}, -1},
		{"elsewhere", wsl, api.AgentInfo{Folder: "/tmp"}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := -1
			for i, wt := range tt.wts {
				if len(owners(wt, tt.wts, []api.AgentInfo{tt.agent})) == 1 {
					if got >= 0 {
						t.Fatalf("the agent belongs to worktrees %d and %d", got, i)
					}
					got = i
				}
			}
			if got != tt.want {
				t.Errorf("belongs to %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWorktreeButtons(t *testing.T) {
	for _, tt := range []struct {
		name   string
		index  int
		remove bool
	}{
		{"main", 1, false}, {"owned", 2, false}, {"unowned", 3, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := worktreeModel(80, 24)
			m.selectIndex(m.entries(), tt.index)
			row := ansi.Strip(m.View()[23])
			if !strings.Contains(row, "Open") || !strings.Contains(row, "Commit...") || strings.Contains(row, "Remove") != tt.remove {
				t.Fatalf("buttons: %q", row)
			}
		})
	}
	for _, tt := range []struct {
		width        int
		open, remove bool
	}{{30, true, true}, {22, true, false}, {17, false, false}} {
		m := worktreeModel(tt.width, 8)
		m.selectIndex(m.entries(), 3)
		row := ansi.Strip(m.View()[7])
		if !strings.Contains(row, "Commit...") || strings.Contains(row, "Open") != tt.open || strings.Contains(row, "Remove") != tt.remove {
			t.Errorf("width %d buttons: %q", tt.width, row)
		}
	}
}

func TestRemoveButtonSameAsKey(t *testing.T) {
	makeModel := func() *Model { m := worktreeModel(80, 24); m.selectIndex(m.entries(), 3); return m }
	click, press := makeModel(), makeModel()
	row := ansi.Strip(click.View()[23])
	x := strings.Index(row, "Remove")
	if x < 0 {
		t.Fatalf("no Remove button: %q", row)
	}
	got, want := click.Click(x, 23), press.Key(key("x"))
	if !reflect.DeepEqual(got, want) || click.removePath != press.removePath || click.removePath == "" {
		t.Fatalf("Remove click differs from x: %#v / %#v", got, want)
	}
	if got, want = click.Key(key("y")), press.Key(key("y")); !reflect.DeepEqual(got, want) {
		t.Fatalf("confirmation differs: %#v / %#v", got, want)
	}
	wantAction(t, got, api.GitActRemoveWorktree, map[string]string{api.ArgGitPath: "/home/u/snow/.worktrees/spike"})
}

// worktrees is a state with a change and a linked worktree that an agent
// works in.
func worktrees() (api.GitState, []api.AgentInfo) {
	st := api.GitState{
		Folder: "/home/u/snow", Branch: "main",
		Changes: []api.GitChange{{Path: "README.md", Work: "M"}},
		Worktrees: []api.GitWorktree{
			{Path: "/home/u/snow", Branch: "main", Main: true},
			{Path: "/home/u/snow/.worktrees/agent-1", Branch: "agent/1"},
			{Path: "/home/u/snow/.worktrees/spike", Detached: true, Locked: true},
		},
	}
	agents := []api.AgentInfo{{Pane: 9, Window: 2, WindowName: "agent/1", Folder: "/home/u/snow/.worktrees/agent-1"}}
	return st, agents
}

func worktreeModel(w, h int) *Model {
	st, agents := worktrees()
	m := New(7, testTheme())
	m.SetFocused(true)
	m.SetAgents(agents)
	m.SetSize(w, h)
	m.SetState(st)
	return m
}

// TestWorktreeEnter: Enter on a worktree an agent works in goes to that
// agent; on any other worktree it opens a terminal there in a new window.
func TestWorktreeEnter(t *testing.T) {
	m := worktreeModel(80, 24)
	m.Key(key("down")) // the main checkout
	m.Key(key("down")) // agent-1
	cmd, ok := m.Key(key("enter")).(*api.Command)
	if !ok || cmd.Op != api.OpFocus || cmd.Target != 9 {
		t.Fatalf("Enter on agent-1 sent %#v, want focus pane 9", cmd)
	}
	m.Key(key("down")) // spike
	cmd, ok = m.Key(key("enter")).(*api.Command)
	if !ok || cmd.Op != api.OpNewWindow || cmd.Args[api.ArgFolder] != "/home/u/snow/.worktrees/spike" || cmd.Args[api.ArgKind] != api.PaneTerminal {
		t.Fatalf("Enter on spike sent %#v, want a terminal in a new window there", cmd)
	}
	if a := m.Key(key("s")); a != nil {
		t.Errorf("s on a worktree sent %#v", a)
	}
}

// TestWorktreeRows: each row shows its folder name, branch or detached,
// its lock, and its owner; "*" marks the pane's own checkout. One checkout
// alone shows no section.
func TestWorktreeRows(t *testing.T) {
	view := ansi.Strip(strings.Join(worktreeModel(80, 24).View(), "\n"))
	for _, want := range []string{"Worktrees (3)", "snow", "main", "agent-1", "agent/1", "window 2", "spike", "detached", "locked"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	st, _ := worktrees()
	m := worktreeModel(80, 24)
	want := [][]string{{"*", "snow", "main", "", ""}, {"", "agent-1", "agent/1", "", "window 2"}, {"", "spike", "detached", "locked", ""}}
	for i, wt := range st.Worktrees {
		got := m.worktreeCols(wt)
		if !slices.Equal(got, want[i]) {
			t.Errorf("worktree %d cells = %q, want %q", i, got, want[i])
		}
	}
	st.Worktrees = st.Worktrees[:1]
	if view := ansi.Strip(strings.Join(model(st, 80, 24).View(), "\n")); strings.Contains(view, "Worktrees") {
		t.Errorf("one checkout shows a section:\n%s", view)
	}
}

// history is a state with no changes and two commits: one an agent made
// as Snow saw, one Snow knows nothing about.
func history() api.GitState {
	return api.GitState{
		Folder: "/home/u/snow", Branch: "main",
		History: []api.GitCommit{
			{Hash: strings.Repeat("a", 40), Short: "aaaaaaa", Author: "Ada", Subject: "feat(git): add the git pane", Agent: "window 2 agent/1"},
			{Hash: strings.Repeat("b", 40), Short: "bbbbbbb", Author: "Ada", Subject: "docs: fix a typo"},
		},
	}
}

// TestHistoryRows: a commit shows its agent only when Snow saw the agent
// make it; Enter on a commit does nothing.
func TestHistoryRows(t *testing.T) {
	m := model(history(), 80, 24)
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	for _, want := range []string{"No changes.", "History (2)", "aaaaaaa  by window 2 agent/1  feat(git): add the git pane", "bbbbbbb  docs: fix a typo"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	if msg := m.Key(key("enter")); msg != nil {
		t.Errorf("Enter on a commit sent %#v", msg)
	}
	if strings.Contains(view, "enter diff") {
		t.Errorf("a commit row offers a diff:\n%s", view)
	}
}

// TestWorktreeRemoveKey asks before removing an unowned linked worktree.
func TestWorktreeRemoveKey(t *testing.T) {
	st := api.GitState{Folder: "/src/snow", Branch: "main", Worktrees: []api.GitWorktree{
		{Path: "/src/snow", Branch: "main", Main: true},
		{Path: "/src/snow/.worktrees/clean", Branch: "clean"},
	}}
	m := model(st, 80, 24)
	m.Key(key("down")) // the unowned linked worktree
	if !m.CanRemoveSelectedWorktree() {
		t.Fatal("selected clean worktree is not removable")
	}
	if msg := m.Key(key("x")); msg != nil || !strings.Contains(ansi.Strip(strings.Join(m.View(), "\n")), "Remove worktree clean?") {
		t.Fatalf("x did not ask for confirmation; message=%#v\n%s", msg, strings.Join(m.View(), "\n"))
	}
	if msg := m.Key(key("y")); msg == nil {
		t.Fatal("y did not send the remove action")
	} else if a, ok := msg.(*api.SidePaneAction); !ok || a.Action != api.GitActRemoveWorktree || a.Args[api.ArgGitPath] != "/src/snow/.worktrees/clean" {
		t.Fatalf("y sent %#v", msg)
	}

	m = model(st, 80, 24)
	m.Key(key("down"))
	m.Key(key("x"))
	if msg := m.Key(key("n")); msg != nil || m.removePath != "" {
		t.Fatalf("n did not cancel: msg=%#v removePath=%q", msg, m.removePath)
	}

	m = model(st, 80, 24)
	m.SetAgents([]api.AgentInfo{{Pane: 9, Folder: "/src/snow/.worktrees/clean"}})
	m.Key(key("down"))
	if msg := m.Key(key("x")); msg != nil || m.removePath != "" {
		t.Fatalf("x on an owned worktree asked to remove it: msg=%#v removePath=%q", msg, m.removePath)
	}
}

// TestWorktreeRemoveHint keeps removal discoverable in the menu while the
// button row replaces key hints. Its button arrives in #484.
func TestWorktreeRemoveHint(t *testing.T) {
	st := api.GitState{Folder: "/src/snow", Branch: "main", Changes: []api.GitChange{{Path: "README.md", Work: "M"}}, Worktrees: []api.GitWorktree{{Path: "/src/snow", Branch: "main", Main: true}, {Path: "/src/snow/.worktrees/clean", Branch: "clean"}}}
	m := model(st, 80, 24)
	hasRemove := func() bool {
		for _, a := range m.MenuActions() {
			if a.Label == "Remove worktree" && a.Shortcut == "x" {
				return true
			}
		}
		return false
	}
	if hasRemove() {
		t.Fatal("change row offers Remove")
	}
	m.Key(key("down"))
	if hasRemove() {
		t.Fatal("main checkout offers Remove")
	}
	m.Key(key("down"))
	if !hasRemove() {
		t.Fatal("unowned worktree lacks Remove menu entry")
	}
	m.SetAgents([]api.AgentInfo{{Pane: 9, Folder: "/src/snow/.worktrees/clean"}})
	if hasRemove() {
		t.Fatal("owned worktree offers Remove")
	}
}

func TestCanRemoveSelectedWorktreeOnlyInListMode(t *testing.T) {
	st := api.GitState{Folder: "/src/snow", Branch: "main", Worktrees: []api.GitWorktree{
		{Path: "/src/snow", Branch: "main", Main: true},
		{Path: "/src/snow/.worktrees/clean", Branch: "clean"},
	}}
	m := model(st, 80, 24)
	m.Key(key("down"))
	if !m.CanRemoveSelectedWorktree() {
		t.Fatal("list mode does not allow removing an unowned linked worktree")
	}
	m.openCommit()
	if m.CanRemoveSelectedWorktree() {
		t.Fatal("commit mode allows the remove menu action")
	}

	st.Diff = &api.GitDiff{Path: "README.md"}
	m = model(st, 80, 24)
	m.selectIndex(m.entries(), 1)
	if m.CanRemoveSelectedWorktree() {
		t.Fatal("diff mode allows the remove menu action")
	}
}
