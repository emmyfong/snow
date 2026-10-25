package chat

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// failures are the errors a turn can end with, with the choices the agent
// offers for each (internal/agent/failure.go).
var failures = map[string]api.ChatError{
	"NoKey":          {Code: api.ChatErrNoKey, Message: "no key saved for anthropic", Choices: []string{"login", "pick-model"}},
	"BadKey":         {Code: api.ChatErrBadKey, Message: "anthropic rejected the key (401)", Choices: []string{"login", "pick-model"}},
	"ModelNotFound":  {Code: api.ChatErrModelNotFound, Message: "model claude-sonnet-4-0 was retired", Choices: []string{"pick-model"}},
	"RateLimited":    {Code: api.ChatErrRateLimited, Message: "rate limit reached for requests", RetryAfter: 41.2, Choices: []string{"wait", "retry", "pick-model"}},
	"Overloaded":     {Code: api.ChatErrOverloaded, Message: "anthropic is overloaded (529)", Choices: []string{"retry", "wait", "pick-model"}},
	"ContextTooLong": {Code: api.ChatErrContextTooLong, Message: "prompt is 212000 tokens; the limit is 200000", Choices: []string{"pick-model"}},
	"Network":        {Code: api.ChatErrNetwork, Message: "connection reset while reading the reply", Choices: []string{"retry"}},
	"Save":           {Code: api.ChatErrSave, Message: "write /home/emmy/.snow/conversations/c1.jsonl: no space left on device", Choices: []string{"retry"}},
	"Other":          {Code: "teapot", Message: "the provider returned 418", Choices: []string{"retry", "pick-model"}},
}

// failTurn plays a turn that streams partial text, then fails with e.
func failTurn(m *Model, e api.ChatError) {
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "Explain the rename command"}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventText, Text: "The rename command changes"}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndFailed, Error: &e}))
}

// TestErrorStatesGolden: one golden per failure, and a turn interrupted
// by a server restart, at 80x24 and 120x40.
func TestErrorStatesGolden(t *testing.T) {
	states := map[string]func(m *Model){}
	for name, e := range failures {
		states[name] = func(m *Model) { failTurn(m, e) }
	}
	states["NoProvider"] = func(m *Model) {
		m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Mode: "ask"})
	}
	states["Restarted"] = func(m *Model) {
		m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Model: "anthropic/claude-sonnet-5-5", Mode: "ask", Turn: 1, Entries: []api.ChatEntry{
			{Kind: api.ChatEntryUser, Turn: 1, Text: "Explain the rename command"},
			{Kind: api.ChatEntryAssistant, Turn: 1, Text: "The rename command changes", Cut: true},
			{Kind: api.ChatEntryInterrupted, Turn: 1},
		}})
	}
	for name, state := range states {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m, _ := newChat(size[0], size[1])
				state(m)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// TestRetryAfterShown: the wait the provider asked for shows, rounded up.
func TestRetryAfterShown(t *testing.T) {
	m, _ := newChat(100, 30)
	failTurn(m, failures["RateLimited"])
	if s := screen(m); !strings.Contains(s, "Rate limited") || !strings.Contains(s, "wait 42s before a retry") {
		t.Fatalf("screen:\n%s", s)
	}
}

// TestCutOffTextMarked: the part of a reply that came before a failure
// stays, marked as cut off.
func TestCutOffTextMarked(t *testing.T) {
	m, _ := newChat(100, 30)
	failTurn(m, failures["Network"])
	if s := screen(m); !strings.Contains(s, "The rename command changes") || !strings.Contains(s, "(cut off)") {
		t.Fatalf("screen:\n%s", s)
	}
}

// TestRetrySendsChatInput: Enter on an empty line, and a click on Retry,
// send a retry; with text typed, Enter sends the message instead.
func TestRetrySendsChatInput(t *testing.T) {
	m, _ := newChat(100, 30)
	failTurn(m, failures["Network"])
	in := m.Key(enter)
	if in == nil || in.Kind != api.ChatInputRetry || in.Pane != 2 || in.Agent != "c1" || in.Validate() != nil {
		t.Fatalf("Enter sent %+v", in)
	}

	m, _ = newChat(100, 30)
	failTurn(m, failures["Network"])
	clicked := false
	for y, r := range m.View() {
		if x := strings.Index(ansi.Strip(r), " Retry "); x >= 0 {
			if in := m.Click(x+1, y); in == nil || in.Kind != api.ChatInputRetry {
				t.Fatalf("click sent %+v", in)
			}
			clicked = true
		}
	}
	if !clicked {
		t.Fatalf("no Retry button:\n%s", screen(m))
	}

	m, _ = newChat(100, 30)
	failTurn(m, failures["Network"])
	typeText(m, "never mind")
	if in := m.Key(enter); in == nil || in.Kind != api.ChatInputMessage || in.Text != "never mind" {
		t.Fatalf("typed Enter sent %+v", in)
	}
}

// TestNoAutoModelSwitch: after each failure, every choice either retries
// or asks the session; nothing changes the model, and the top bar still
// shows the model the chat had.
func TestNoAutoModelSwitch(t *testing.T) {
	for name, e := range failures {
		t.Run(name, func(t *testing.T) {
			m, _ := newChat(140, 30)
			failTurn(m, e)
			for range errorChoices(&e) {
				for _, k := range []tea.Key{enter, right} {
					if in := m.Key(k); in != nil && in.Kind != api.ChatInputRetry {
						t.Fatalf("%v sent %+v", k, in)
					}
					if a := m.Asked(); a != "" && a != AskLogin && a != AskPickModel {
						t.Fatalf("asked %q", a)
					}
				}
			}
			if m.Chat().Model != "anthropic/claude-sonnet-5-5" || !strings.Contains(stripped(m.View()[0]), "anthropic/claude-sonnet-5-5") {
				t.Fatalf("model %q, bar %q", m.Chat().Model, stripped(m.View()[0]))
			}
		})
	}
}

// TestErrorChoicesAsk: Log in and Pick another model become requests for
// the session.
func TestErrorChoicesAsk(t *testing.T) {
	m, _ := newChat(100, 30)
	failTurn(m, failures["BadKey"])
	if in := m.Key(enter); in != nil || m.Asked() != AskLogin {
		t.Fatalf("log in sent %+v", in)
	}
	m.Key(right)
	if m.Key(enter); m.Asked() != AskPickModel {
		t.Fatal("pick another model raised nothing")
	}
	if m.Asked() != "" {
		t.Fatal("Asked did not clear")
	}
}

// TestOldErrorsTakeNoKeys: once a new turn starts, an earlier failure's
// box loses its choices and Enter sends nothing for it.
func TestOldErrorsTakeNoKeys(t *testing.T) {
	m, _ := newChat(100, 30)
	failTurn(m, failures["Network"])
	oneTurn(m, 2, "try again", "Done.")
	if in := m.Key(enter); in != nil {
		t.Fatalf("Enter sent %+v", in)
	}
	if strings.Contains(screen(m), " Retry ") {
		t.Fatalf("old box kept its buttons:\n%s", screen(m))
	}
}
