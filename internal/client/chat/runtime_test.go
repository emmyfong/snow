package chat

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestRuntimeApprovalOnlyOffersOneTimeDecisions(t *testing.T) {
	m := waiting(t, 80, 24)
	m.snap.Runtime = &api.RuntimeInfo{Kind: "claude-code"}
	if strings.Contains(screen(m), "Always") {
		t.Fatal("runtime offers an unsupported persistent grant")
	}
	if in := m.Key(letter('a')); in != nil {
		t.Fatalf("unsupported Always shortcut sent %+v", in)
	}
	if in := typeKeys(m, right, enter, enter); in == nil || in.Decision != api.ChatDecisionNo {
		t.Fatalf("second runtime choice did not deny: %+v", in)
	}
}

func TestRuntimeRunControlsKeyboardAndMouse(t *testing.T) {
	m, _ := newChat(80, 24)
	m.snap.Runtime = &api.RuntimeInfo{Kind: "claude-code", Auth: "subscription", Environment: "This machine"}
	m.Key(key('t', tea.ModCtrl, ""))
	if m.Asked() != AskRunControls {
		t.Fatal("Ctrl+T did not open run controls")
	}
	for y, row := range m.View() {
		if strings.Contains(ansi.Strip(row), "Run controls") {
			m.Click(3, y)
			if m.Asked() != AskRunControls {
				t.Fatal("click did not open run controls")
			}
			return
		}
	}
	t.Fatal("run controls were not discoverable")
}

func TestRuntimeSendNowPreservesInvalidDraft(t *testing.T) {
	m, _ := newChat(80, 24)
	for _, text := range []string{" ", strings.Repeat("x", api.MaxChatMessage+1)} {
		m.setInput(text)
		if in := m.SendNowInput(); in != nil || string(m.input) != text {
			t.Fatal("invalid draft was lost or sent")
		}
	}
	m.setInput("exact\nrequest")
	in := m.SendNowInput()
	if in == nil || in.Kind != api.ChatInputSendNow || in.Text != "exact\nrequest" || len(m.input) != 0 {
		t.Fatalf("send now: %+v", in)
	}
}

func TestRuntimeNeedsYouGolden(t *testing.T) {
	for _, state := range []string{"approval", "question", "answered", "stopping", "stop-failed"} {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", state, size[0], size[1]), func(t *testing.T) {
				m := waiting(t, size[0], size[1])
				m.snap.Model = "claude-code/default"
				m.Event(ev(api.ChatEvent{Kind: api.ChatEventRuntime, Runtime: &api.RuntimeInfo{Kind: "claude-code", Auth: "subscription", Environment: "This machine", Session: "session-1"}}))
				m.Event(ev(api.ChatEvent{Kind: api.ChatEventUsage, Usage: &api.ChatUsage{Input: 10, Output: 5, Scope: "main-agent", Cumulative: true, Cost: 0.03, Priced: true}}))
				if state != "approval" {
					m.Event(ev(api.ChatEvent{Kind: api.ChatEventApproval}))
				}
				switch state {
				case "question":
					m.Event(ev(api.ChatEvent{Kind: api.ChatEventQuestion, Question: &api.ChatQuestion{ID: "q1", Text: "Which scope should I use?", Options: []api.ChatOption{{Label: "Small change"}, {Label: "Broader redesign"}}}}))
				case "answered":
					m.Event(ev(api.ChatEvent{Kind: api.ChatEventAnswered, Question: &api.ChatQuestion{ID: "q1"}, Text: "Which scope should I use?\nSmall change"}))
				case "stopping", "stop-failed":
					m.Event(ev(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStopping}))
				}
				if state == "stop-failed" {
					m.Event(ev(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrRuntimeStop, Message: "Claude did not confirm all owned work stopped. Check its tasks in the selected environment before closing."}}))
				}
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}
