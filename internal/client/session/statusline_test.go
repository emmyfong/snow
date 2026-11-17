package session

import (
	"strings"
	"testing"

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
}
