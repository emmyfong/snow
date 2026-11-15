package session

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func controlsScreen(t *testing.T, cols, rows int, state string) (*Model, *recorder) {
	t.Helper()
	m, r := runtimeModelScreen(t, cols, rows)
	snap := api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "claude-code/default", State: api.ChatStateStreaming, Running: true,
		Runtime: &api.RuntimeInfo{Kind: "claude-code", Auth: "subscription", Environment: "This machine", Connection: "selected",
			Tasks: []api.RuntimeTask{{ID: "child", Run: "run-1", Description: "Review changes", State: "running"}, {ID: "sibling", Run: "run-2", Description: "Run tests", State: "running"}}}}
	switch state {
	case "paused":
		snap.Running = false
		snap.State = api.ChatStateStopped
		snap.Queue = []string{"Check the diff"}
		snap.QueuePaused = true
		snap.Runtime.Tasks = nil
	case "idle":
		snap.Running = false
		snap.State = api.ChatStateIdle
		snap.Runtime.Tasks = nil
	}
	m.Update(&snap)
	return m, r
}

func lastRunInput(r *recorder) *api.ChatInput {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if input, ok := r.sent[i].(*api.ChatInput); ok {
			return input
		}
	}
	return nil
}

func TestRuntimeRunControls(t *testing.T) {
	for _, tc := range []struct {
		name, state   string
		moves         int
		kind, id, run string
	}{
		{"stop root", "running", 0, api.ChatInputInterrupt, "", ""},
		{"stop child", "running", 2, api.ChatInputStopTask, "child", "run-1"},
		{"release drafts", "paused", 0, api.ChatInputResumeQueue, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, r := controlsScreen(t, 80, 24, tc.state)
			m.Update(press('t', tea.ModCtrl, ""))
			if m.over == nil {
				t.Fatal("Ctrl+T did not open run controls")
			}
			for range tc.moves {
				m.Update(press(tea.KeyDown, 0, ""))
			}
			m.Update(press(tea.KeyEnter, 0, ""))
			in := lastRunInput(r)
			if in == nil || in.Kind != tc.kind || in.ID != tc.id || in.Run != tc.run || in.Agent != "c1" {
				t.Fatalf("wrong control: %+v", in)
			}
		})
	}
}

func TestRuntimeRunControlMouse(t *testing.T) {
	m, r := controlsScreen(t, 80, 24, "running")
	m.openRuntimeControls(2)
	for y, row := range strings.Split(content(m), "\n") {
		plain := ansi.Strip(row)
		if x := strings.Index(plain, "Stop task: Review changes"); x >= 0 {
			m.Update(tea.MouseClickMsg(tea.Mouse{X: ansi.StringWidth(plain[:x]) + 1, Y: y, Button: tea.MouseLeft}))
			in := lastRunInput(r)
			if in == nil || in.Kind != api.ChatInputStopTask || in.ID != "child" || in.Run != "run-1" {
				t.Fatalf("mouse sent %+v", in)
			}
			return
		}
	}
	t.Fatal("child control missing")
}

func TestRuntimeInterruptAndSend(t *testing.T) {
	m, r := controlsScreen(t, 80, 24, "running")
	m.Update(tea.PasteMsg{Content: "original\nnew direction"})
	m.openRuntimeControls(2)
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	in := lastRunInput(r)
	if in == nil || in.Kind != api.ChatInputSendNow || in.Text != "original\nnew direction" {
		t.Fatalf("send now: %+v", in)
	}
}

func TestRuntimeControlsGolden(t *testing.T) {
	for _, state := range []string{"running", "paused", "idle"} {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", state, size[0], size[1]), func(t *testing.T) {
				m, _ := controlsScreen(t, size[0], size[1], state)
				m.openRuntimeControls(2)
				golden.RequireEqual(t, content(m))
			})
		}
	}
}

func TestCloseRuntimeChatCancel(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mouse=%v", mouse), func(t *testing.T) {
			m, r := controlsScreen(t, 80, 24, "running")
			m.focused().AgentState = api.AgentWorking
			m.confirmClosePane()
			if !strings.Contains(ansi.Strip(content(m)), "Stop and close") {
				t.Fatal("active runtime close did not ask to stop")
			}
			if mouse {
				clickLabel(t, m, "[ Cancel ]", leftClick)
			} else {
				m.Update(press(tea.KeyEscape, 0, ""))
			}
			if m.confirm != nil || lastRunInput(r) != nil {
				t.Fatal("cancelling interrupted the run")
			}
			for _, message := range r.sent {
				if _, ok := message.(*api.Command); ok {
					t.Fatal("cancelling sent close")
				}
			}
		})
	}
}

func TestRuntimeCloseConfirmGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m, _ := controlsScreen(t, size[0], size[1], "running")
			m.focused().AgentState = api.AgentWorking
			m.confirmClosePane()
			golden.RequireEqual(t, content(m))
		})
	}
}

func TestRuntimeLogoutNeverSelectsUnrelatedProviderKey(t *testing.T) {
	m, r := runtimeModelScreen(t, 80, 24)
	m.authList = api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Connected: true}}}
	typeSlash(m, "/logout")
	if req := lastRuntimeRequest(r); req == nil || req.Action != api.RuntimeList {
		t.Fatal("runtime logout did not open its connection controls")
	}
	for _, message := range r.sent {
		switch message.(type) {
		case *api.AuthListRequest, *api.LogoutRequest:
			t.Fatal("runtime logout entered the direct-API key removal flow")
		}
	}
}
