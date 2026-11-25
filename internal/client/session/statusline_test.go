package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

func statusLineScreen(t *testing.T) *Model {
	t.Helper()
	m := attached(t, 80, 24)
	m.layout.Windows[0].Panes[0].Folder = "/home/a/Projects/snow"
	return m
}

func TestStatusLineContent(t *testing.T) {
	m := statusLineScreen(t)
	got := strings.TrimSpace(lastBarRow(m))
	if got != "work · Projects/snow" {
		t.Fatalf("status line %q", got)
	}
}

func TestStatusLineNeedsYouCount(t *testing.T) {
	m := statusLineScreen(t)
	for _, tt := range []struct {
		agents []api.AgentInfo
		want   string
	}{
		{nil, ""}, {[]api.AgentInfo{{State: api.AgentNeedsYou}, {State: api.AgentWorking}}, "1 need you"},
	} {
		m.agents = tt.agents
		if got := m.statusOptions().Right; got != tt.want {
			t.Fatalf("right %q,want %q", got, tt.want)
		}
		if tt.want != "" && !strings.Contains(lastBarRow(m), tt.want) {
			t.Fatal("count missing from view")
		}
	}
}

func TestStatusLineHintOnlyAtPrompt(t *testing.T) {
	m := statusLineScreen(t)
	if got := m.statusOptions().Hint; got != "" {
		t.Fatalf("idle hint %q", got)
	}
	m.openPrompt()
	if got := m.statusOptions().Hint; got == "" || !strings.Contains(lastBarRow(m), got) {
		t.Fatal("command prompt hint missing")
	}
	m.prompt = nil
	m.startRename()
	if got := m.statusOptions(); got.Prompt == "" || got.Hint == "" {
		t.Fatalf("rename prompt %+v", got)
	}
	if got := lastBarRow(m); !strings.Contains(got, m.renamePrompt()) || !strings.Contains(got, promptHint) {
		t.Fatalf("rename bar lacks prompt or hint: %q", got)
	}
}

// TestCopySearchPromptKeepsMode: the search prompt line keeps the scroll mode
// text at its right end. That text names the exit key, so the prompt hint
// gives way to it.
func TestCopySearchPromptKeepsMode(t *testing.T) {
	m, _, _ := inCopyMode(t, 80, 24)
	m.Update(press('?', 0, "?"))
	got := lastBarRow(m)
	if !strings.Contains(got, m.copy.Prompt()) || !strings.HasSuffix(got, scrollModeLabel) {
		t.Fatalf("copy search bar %q, want the prompt and the scroll mode text", got)
	}
	if strings.Contains(got, promptHint) {
		t.Fatalf("copy search bar %q kept the hint beside the scroll mode text", got)
	}
}

// TestCopyModeShowsExitKey: scroll mode names itself and its exit key at both
// sizes, keeps that text while the search prompt is open, and loses it when
// scroll mode closes.
func TestCopyModeShowsExitKey(t *testing.T) {
	for _, size := range []struct{ cols, rows int }{{80, 24}, {120, 40}} {
		m, _, _ := inCopyMode(t, size.cols, size.rows)
		if got := lastBarRow(m); !strings.Contains(got, "Scroll mode") || !strings.Contains(got, "Esc: exit") {
			t.Fatalf("%dx%d: scroll mode bar %q, want the mode and its exit key", size.cols, size.rows, got)
		}
		m.Update(press('?', 0, "?"))
		if got := lastBarRow(m); !strings.Contains(got, "Scroll mode") || !strings.Contains(got, "Esc: exit") {
			t.Fatalf("%dx%d: search prompt bar %q, want the mode and its exit key", size.cols, size.rows, got)
		}
		m.Update(press(tea.KeyEscape, 0, "")) // closes the prompt
		m.Update(press(tea.KeyEscape, 0, "")) // closes scroll mode
		if m.copy != nil {
			t.Fatalf("%dx%d: Esc left scroll mode open", size.cols, size.rows)
		}
		if got := lastBarRow(m); strings.Contains(got, "Scroll mode") || strings.Contains(got, "Esc: exit") {
			t.Fatalf("%dx%d: closed bar %q still names scroll mode", size.cols, size.rows, got)
		}
	}
}
