package agents

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// base is the time the tests see as now.
var base = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func key(code rune) tea.Key   { return tea.Key{Code: code} }
func text(s string) tea.Key   { return tea.Key{Code: []rune(s)[0], Text: s} }
func clock() func() time.Time { return func() time.Time { return base } }

// three is session "work" with a Claude Code agent that is idle, a chat
// that works, and a Claude Code agent that needs the user, plus an agent
// of another session.
func three() []api.AgentInfo {
	return []api.AgentInfo{
		{Session: "other", Window: 0, WindowName: "sh", Pane: 9, Kind: api.AgentKindClaudeCode, State: api.AgentNeedsYou, Since: base.Add(-time.Hour)},
		{Session: "work", Window: 1, WindowName: "planner", Pane: 2, Kind: api.AgentKindClaudeCode, Folder: "/home/em/snow", State: api.AgentIdle, Since: base.Add(-3 * time.Minute)},
		{Session: "work", Window: 2, WindowName: "chat", Pane: 4, Kind: api.AgentKindChat, Folder: "/home/em/snow/.worktrees/fix-12", State: api.AgentWorking, Since: base.Add(-40 * time.Second), Model: "anthropic/claude-sonnet-5-5", Cost: 0.88},
		{Session: "work", Window: 3, WindowName: "reviewer", Pane: 6, Kind: api.AgentKindClaudeCode, Folder: `C:\src\snow`, State: api.AgentNeedsYou, Since: base.Add(-2 * time.Hour)},
	}
}

func board(usage bool) *Board {
	b := NewBoard("work", usage, clock())
	b.SetAgents(three())
	return b
}

// lightTheme is a theme whose colors read on a light terminal.
func lightTheme() theme.Theme {
	th := theme.Default()
	th.Name = "Light"
	th.Border = color.RGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff}
	th.FocusBorder = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Accent = color.RGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff}
	th.Muted = color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}
	th.Selection = color.RGBA{R: 0xbb, G: 0xd6, B: 0xff, A: 0xff}
	th.SelectionFG = color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	th.PromptFG = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	th.PromptBG = color.RGBA{R: 0xb0, G: 0x40, B: 0x00, A: 0xff}
	return th
}

// TestAgentsBoardGolden draws three agents of a session in idle, working,
// and needs-you states, at two sizes and in a dark and a light theme.
func TestAgentsBoardGolden(t *testing.T) {
	for _, th := range []theme.Theme{theme.Default(), lightTheme()} {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", th.Name, size[0], size[1]), func(t *testing.T) {
				box, x, y := board(true).View(th, size[0], size[1])
				if x != 0 || y != 0 {
					t.Fatalf("board at %d,%d, want the whole window area", x, y)
				}
				if w, h := sizeOf(box); w != size[0] || h != size[1] {
					t.Fatalf("board is %dx%d, want %dx%d", w, h, size[0], size[1])
				}
				golden.RequireEqual(t, box)
			})
		}
	}
}

func sizeOf(box string) (w, h int) {
	lines := strings.Split(box, "\n")
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	return w, len(lines)
}

// TestAgentsBoardRows: the board lists only its session's agents, says
// what each does and for how long, sums the cost, and preselects the agent
// that needs the user.
func TestAgentsBoardRows(t *testing.T) {
	b := board(true)
	box, _, _ := b.View(theme.Default(), 120, 40)
	s := ansi.Strip(box)
	for _, want := range []string{
		"Agents in work", "3 agents, 1 needs you, $0.88 est.",
		"1 planner", "Claude Code", "○ idle", "3m", "snow",
		"2 chat", "◐ working", "40s", "fix-12", "anthropic/claude-sonnet-5-5  $0.88",
		"3 reviewer", "● needs you", "2h", "waiting", // Claude Code does not say what it waits for
	} {
		if !strings.Contains(s, want) {
			t.Errorf("board lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "0 sh") {
		t.Errorf("board shows another session's agent:\n%s", s)
	}
	if pane, ok := b.Choice(); !ok || pane != 6 {
		t.Fatalf("preselected pane %d, want 6, the agent that needs you", pane)
	}
	if hidden, _, _ := board(false).View(theme.Default(), 120, 40); strings.Contains(ansi.Strip(hidden), "$") {
		t.Fatalf("board shows a cost with usage off:\n%s", ansi.Strip(hidden))
	}
}

// TestAgentsBoardJump: Enter accepts the selected row, and Choice names
// that agent's pane, which the session screen focuses.
func TestAgentsBoardJump(t *testing.T) {
	tests := []struct {
		name   string
		keys   []tea.Key
		result picker.Result
		pane   int
	}{
		{"enter on the preselected agent", []tea.Key{key(tea.KeyEnter)}, picker.Accepted, 6},
		{"up then enter", []tea.Key{key(tea.KeyUp), key(tea.KeyEnter)}, picker.Accepted, 4},
		{"j wraps to the first", []tea.Key{text("j"), key(tea.KeyEnter)}, picker.Accepted, 2},
		{"home", []tea.Key{key(tea.KeyHome)}, picker.Open, 2},
		{"esc closes", []tea.Key{key(tea.KeyEscape)}, picker.Cancelled, 6},
		{"q closes", []tea.Key{text("q")}, picker.Cancelled, 6},
	}
	for _, tt := range tests {
		b := board(true)
		var r picker.Result
		for _, k := range tt.keys {
			r = b.Key(k)
		}
		if pane, _ := b.Choice(); r != tt.result || pane != tt.pane {
			t.Errorf("%s: result %v pane %d, want %v %d", tt.name, r, pane, tt.result, tt.pane)
		}
	}
}

// TestAgentsBoardClick: a click on a row chooses its agent.
func TestAgentsBoardClick(t *testing.T) {
	b := board(true)
	th := theme.Default()
	box, _, _ := b.View(th, 80, 24)
	row := -1
	for i, l := range strings.Split(ansi.Strip(box), "\n") {
		if strings.Contains(l, "planner") {
			row = i
		}
	}
	if r := b.Click(th, 80, 24, 10, row); r != picker.Accepted {
		t.Fatalf("click on the planner row gave %v, want Accepted", r)
	}
	if pane, _ := b.Choice(); pane != 2 {
		t.Fatalf("clicked pane %d, want 2", pane)
	}
	if r := b.Click(th, 80, 24, 10, 1); r != picker.Open {
		t.Fatalf("click on the summary gave %v, want Open", r)
	}
}

// TestAgentsBoardUpdatesLive: a new list changes a row's state at once,
// and the selection stays on its agent.
func TestAgentsBoardUpdatesLive(t *testing.T) {
	b := board(true)
	b.Key(key(tea.KeyUp)) // the chat, pane 4
	list := three()
	list[2].State, list[2].Since = api.AgentNeedsYou, base
	list[2].Waits = api.AgentWaitsQuestion
	list[3].State = api.AgentIdle
	b.SetAgents(list)
	s := ansi.Strip(func() string { box, _, _ := b.View(theme.Default(), 120, 40); return box }())
	if !strings.Contains(s, "question") || strings.Contains(s, "◐ working") {
		t.Fatalf("board after the update:\n%s", s)
	}
	if pane, _ := b.Choice(); pane != 4 {
		t.Fatalf("selection moved to pane %d, want it kept on 4", pane)
	}
	b.SetAgents(list[:2]) // the chat's pane closed
	if pane, ok := b.Choice(); !ok || pane != 2 {
		t.Fatalf("selection %d %v after the agent left, want the first agent", pane, ok)
	}
}

// TestAgentsBoardEmpty: a session with no agent says so, and Enter closes.
func TestAgentsBoardEmpty(t *testing.T) {
	b := NewBoard("work", true, clock())
	b.SetAgents(nil)
	box, _, _ := b.View(theme.Default(), 80, 24)
	if !strings.Contains(ansi.Strip(box), "No agents in this session.") {
		t.Fatalf("empty board:\n%s", ansi.Strip(box))
	}
	if r := b.Key(key(tea.KeyEnter)); r != picker.Cancelled {
		t.Fatalf("Enter on an empty board gave %v, want Cancelled", r)
	}
	if box, _, _ := b.View(theme.Default(), 20, 5); box != "" {
		t.Fatal("board drew in a window too small for it")
	}
}

func TestSpanAndFolder(t *testing.T) {
	for d, want := range map[time.Duration]string{-time.Second: "0s", 12 * time.Second: "12s", 3 * time.Minute: "3m", 2 * time.Hour: "2h", 50 * time.Hour: "2d"} {
		if got := span(d); got != want {
			t.Errorf("span(%v) = %q, want %q", d, got, want)
		}
	}
	for path, want := range map[string]string{"/home/em/snow": "snow", `C:\src\app\`: "app", "": "", "snow": "snow"} {
		if got := folderName(path); got != want {
			t.Errorf("folderName(%q) = %q, want %q", path, got, want)
		}
	}
}
