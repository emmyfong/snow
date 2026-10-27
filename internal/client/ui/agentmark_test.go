package ui

import (
	"fmt"
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
)

// agentTabs is a bar with each agent state, one waiting window that is
// shown, and a shell window with no agent.
var agentTabs = []Tab{
	{Index: 0, Name: "agents", Agent: MarkIdle},
	{Index: 1, Name: "planner", Agent: MarkWorking},
	{Index: 2, Name: "builder", Agent: MarkNeedsYou},
	{Index: 3, Name: "reviewer", Agent: MarkNeedsYou, Active: true},
	{Index: 4, Name: "terminal"},
}

// TestStatusBarAgentStates: each state shows its glyph after the window's
// name, and a window that needs the user stands out, in every built-in
// theme. The bar draws on its own token colors, not the terminal's
// background, so one golden per theme covers light and dark terminals.
func TestStatusBarAgentStates(t *testing.T) {
	for _, th := range theme.Builtin() {
		t.Run(strings.ReplaceAll(th.Name, " ", "_"), func(t *testing.T) {
			golden.RequireEqual(t, StatusBar(th, StatusOptions{Width: 100, Session: "rename-fix", Tabs: agentTabs, Right: "14:02"}))
		})
	}
	got := ansi.Strip(StatusBar(theme.Default(), StatusOptions{Width: 100, Session: "s", Tabs: agentTabs}))
	for _, want := range []string{"0 agents ○", "1 planner ◐", "2 builder ●", "3 reviewer* ●", "4 terminal "} {
		if !strings.Contains(got, want) {
			t.Errorf("bar %q lacks %q", got, want)
		}
	}
}

// TestNeedsYouStandsOut: a waiting window uses the prompt colors, unlike a
// plain tab and the shown window's tab.
func TestNeedsYouStandsOut(t *testing.T) {
	th := theme.Default()
	tab := func(tb Tab) string {
		return StatusBar(th, StatusOptions{Width: 40, Session: "s", Tabs: []Tab{tb}})
	}
	plain := tab(Tab{Index: 1, Name: "dev", Agent: MarkWorking})
	waiting := tab(Tab{Index: 1, Name: "dev", Agent: MarkNeedsYou})
	active := tab(Tab{Index: 1, Name: "dev", Active: true})
	if waiting == plain || waiting == active {
		t.Fatal("a window that needs you looks like another tab")
	}
	if !strings.Contains(waiting, colorCode(th.PromptBG)) {
		t.Fatalf("waiting tab %q is not in the prompt colors", waiting)
	}
}

// TestTabSpansWithStates: spans cover the glyphs, so a click on one still
// finds its window, also when the bar is cut short.
func TestTabSpansWithStates(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  []string
	}{
		{"all fit", 100, []string{" 0 agents ○ ", " 1 planner ◐ ", " 2 builder ● ", " 3 reviewer* ● ", " 4 terminal "}},
		{"cut short", 40, []string{" 0 agents ○ ", " 1 planner ◐ ", " 2 builder"}},
	}
	for _, tt := range tests {
		o := StatusOptions{Width: tt.width, Session: "s", Tabs: agentTabs}
		bar := ansi.Strip(StatusBar(theme.Default(), o))
		var got []string
		for _, sp := range TabSpans(theme.Default(), o) {
			got = append(got, ansi.Cut(bar, sp.X, sp.X+sp.W))
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: text under spans = %q, want %q (bar %q)", tt.name, got, tt.want, bar)
		}
	}
}

// TestStatusBarNoAgentUnchanged: a window with no agent draws exactly as
// before agent marks existed: no glyph, no extra cell.
func TestStatusBarNoAgentUnchanged(t *testing.T) {
	for _, th := range theme.Builtin() {
		o := StatusOptions{Width: 60, Session: "api", Tabs: []Tab{{Index: 0, Name: "dev"}, {Index: 1, Name: "logs", Active: true}}, Right: "14:02"}
		got := ansi.Strip(StatusBar(th, o))
		if want := " [api]  0 dev  1 logs* "; !strings.HasPrefix(got, want) {
			t.Errorf("%s: bar %q, want it to start %q", th.Name, got, want)
		}
	}
}

// TestAgentMarkPlainFallback: a terminal with no colors gets ASCII marks.
func TestAgentMarkPlainFallback(t *testing.T) {
	th := theme.Default()
	th.Glyphs.AgentIdle, th.Glyphs.AgentWorking, th.Glyphs.AgentNeedsYou = "-", "~", "!"
	got := ansi.Strip(StatusBar(th, StatusOptions{Width: 100, Session: "s", Tabs: agentTabs}))
	for _, want := range []string{"0 agents -", "1 planner ~", "2 builder !"} {
		if !strings.Contains(got, want) {
			t.Errorf("bar %q lacks %q", got, want)
		}
	}
}

// colorCode is the escape code parameters that set c as the background.
func colorCode(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)
}
