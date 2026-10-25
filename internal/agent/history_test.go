package agent

import (
	"encoding/json"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// validExchange fails unless every tool call in msgs is answered by a
// result in the next message, as providers require.
func validExchange(t *testing.T, msgs []provider.Message) {
	t.Helper()
	for i, m := range msgs {
		for _, b := range m.Content {
			u, ok := b.(*provider.ToolUse)
			if !ok {
				continue
			}
			found := false
			if i+1 < len(msgs) {
				for _, nb := range msgs[i+1].Content {
					if r, ok := nb.(*provider.ToolResult); ok && r.ToolUseID == u.ID {
						found = true
					}
				}
			}
			if !found {
				dump, _ := json.Marshal(msgs)
				t.Fatalf("tool call %s has no result in the next message: %s", u.ID, dump)
			}
		}
	}
}

// TestInterruptThenSendKeepsExchangeValid: after an interrupt during a
// tool, an approval wait, or a stream, the next request is one a provider
// accepts, and it matches what a resume would send.
func TestInterruptThenSendKeepsExchangeValid(t *testing.T) {
	waiting := providertest.Reply{Events: []provider.Event{&provider.TextDelta{Text: "Hal"}}, Wait: true}
	tests := []struct {
		name  string
		tools func() []tool.Tool
		first providertest.Reply
		wait  func(*testing.T, *harness)
	}{
		{"running tool", func() []tool.Tool {
			return []tool.Tool{gate(make(chan struct{}), make(chan error, 1))}
		}, call("t1", "gate"), func(t *testing.T, h *harness) { waitState(t, h, StateTool) }},
		{"approval wait", func() []tool.Tool {
			return []tool.Tool{changer(make(chan struct{}, 1))}
		}, call("t1", "change"), func(t *testing.T, h *harness) { next[*ApprovalRequest](t, h) }},
		{"stream", func() []tool.Tool { return nil }, waiting, func(t *testing.T, h *harness) { next[*TextDelta](t, h) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, tt.tools(), tt.first, text("ok"))
			_ = h.conv.Send("go")
			tt.wait(t, h)
			_ = h.conv.Interrupt()
			next[*TurnEnded](t, h)
			_ = h.conv.Send("again")
			if end := next[*TurnEnded](t, h); end.Reason != EndDone {
				t.Fatalf("second turn ended %+v", end)
			}
			reqs := h.fake.Requests()
			live := reqs[len(reqs)-1].Messages
			validExchange(t, live)
			c, err := h.store.Load(h.conv.ID())
			if err != nil {
				t.Fatal(err)
			}
			// The file also holds the second reply, which the request did not.
			resumed := history(c.Records)
			if len(resumed) != len(live)+1 {
				a, _ := json.Marshal(live)
				b, _ := json.Marshal(resumed)
				t.Fatalf("live and resumed history differ:\nlive    %s\nresumed %s", a, b)
			}
		})
	}
}

// TestToolCallWithOtherStopIsAnswered: a reply cut off by max tokens that
// still holds a whole tool call leaves the next request valid.
func TestToolCallWithOtherStopIsAnswered(t *testing.T) {
	cut := providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: "t1", Name: "echo", Input: json.RawMessage(`{}`)},
		&provider.Stop{Reason: provider.StopMaxTokens},
	}}
	h := newHarness(t, []tool.Tool{echo}, cut, text("ok"))
	_ = h.conv.Send("go")
	next[*TurnEnded](t, h)
	_ = h.conv.Send("again")
	next[*TurnEnded](t, h)
	reqs := h.fake.Requests()
	validExchange(t, reqs[len(reqs)-1].Messages)
}

// waitState waits for a StateChanged event to state.
func waitState(t *testing.T, h *harness, state State) {
	t.Helper()
	for {
		if next[*StateChanged](t, h).State == state {
			return
		}
	}
}
