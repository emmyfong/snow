package chat

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// runSlash types text and runs it as a command with Enter.
func runSlash(m *Model, text string) *api.ChatInput {
	typeText(m, text)
	return m.Key(key(tea.KeyEnter, 0, ""))
}

// TestModelCommandAsksForThePicker: /model with no argument asks the
// session for the picker instead of sending anything.
func TestModelCommandAsksForThePicker(t *testing.T) {
	m, _ := newChat(80, 24)
	if in := runSlash(m, "/model"); in != nil {
		t.Fatalf("/model sent %+v", in)
	}
	if got := m.Asked(); got != AskPickModel {
		t.Fatalf("Asked = %q, want %q", got, AskPickModel)
	}
}

// TestModelCommandSwitches: /model provider/model makes the next turn use
// that model, keeps the effort, and does not save a default.
func TestModelCommandSwitches(t *testing.T) {
	m, _ := newChat(80, 24)
	in := runSlash(m, "/model anthropic/claude-opus-5-5")
	if in == nil || in.Kind != api.ChatInputModel || in.Model != "anthropic/claude-opus-5-5" || in.Effort != "" || in.SaveDefault {
		t.Fatalf("/model sent %+v", in)
	}
	if in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("/model sent to pane %d agent %q", in.Pane, in.Agent)
	}
}

// TestModelCommandUsesTheChatProvider: a bare model ID takes the chat's
// provider, and the chat's effort stays.
func TestModelCommandUsesTheChatProvider(t *testing.T) {
	m, _ := newChat(80, 24)
	m.snap.Effort = "high"
	in := runSlash(m, "/model claude-haiku-5-5")
	if in == nil || in.Model != "anthropic/claude-haiku-5-5" || in.Effort != "high" {
		t.Fatalf("/model with a bare ID sent %+v", in)
	}
}

// TestModelCommandRejectsBadInput: two words, or a bare ID with no current
// provider, leaves the input alone and says what to type.
func TestModelCommandRejectsBadInput(t *testing.T) {
	for _, tt := range []struct {
		name, text, model string
	}{
		{"two words", "/model a b", "anthropic/claude-sonnet-5-5"},
		{"no provider", "/model model-1", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newChat(80, 24)
			m.snap.Model = tt.model
			if in := runSlash(m, tt.text); in != nil {
				t.Fatalf("%s sent %+v", tt.text, in)
			}
			if note := m.Note(); !strings.Contains(note, "/model provider/model") {
				t.Fatalf("note = %q", note)
			}
		})
	}
}

// TestNoModelRefusesTheMessage: a chat without a model sends nothing and
// says how to choose one.
func TestNoModelRefusesTheMessage(t *testing.T) {
	m, _ := newChat(80, 24)
	m.snap.Model = ""
	m.SetProviderReady(true)
	typeText(m, "hello")
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("a message without a model sent %+v", in)
	}
	if note := m.Note(); !strings.Contains(note, "Choose a model first") {
		t.Fatalf("note = %q", note)
	}
	if got := string(m.input); got != "hello" {
		t.Fatalf("the input was cleared: %q", got)
	}
}

// TestModelClickAsksForThePicker: a click on the top bar's model asks the
// session for the picker, and a click elsewhere on the bar does not.
func TestModelClickAsksForThePicker(t *testing.T) {
	m, _ := newChat(120, 30)
	_, spans := ui.TopBarRow(m.th, m.barOptions())
	for _, s := range spans {
		if s.Kind == ui.TopBarModel {
			if in := m.Click(s.X+1, 0); in != nil {
				t.Fatalf("the model click sent %+v", in)
			}
			if got := m.Asked(); got != AskPickModel {
				t.Fatalf("Asked = %q, want %q", got, AskPickModel)
			}
			return
		}
	}
	t.Fatal("the top bar has no model span")
}
