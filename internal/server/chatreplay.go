package server

import (
	"strings"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// replaySnapshot builds the snapshot of chat pane id for a resumed
// conversation: the records play again as the events a live turn made, so
// the transcript is the one the user saw, and the pane is idle. The model,
// effort, and mode are the ones the conversation starts with, which may
// differ from the file's last (see resumeChat).
func replaySnapshot(id int, conv *agent.Conversation, loaded *convstore.Conversation, model, effort string, mode agent.Mode) api.ChatSnapshot {
	snap := newSnapshot(id, conv, model, effort, mode)
	cur := loaded.Meta.Model
	turn, running := 0, false
	play := func(b agent.Body) {
		if e := chatEvent(id, agent.Event{Agent: conv.Agent(), Body: b}); e != nil {
			snap.Apply(e)
		}
	}
	end := func(r agent.EndReason, f *agent.Failure) {
		if running {
			play(&agent.TurnEnded{Turn: turn, Reason: r, Failure: f})
			running = false
		}
	}
	for _, rec := range loaded.Records {
		switch r := rec.(type) {
		case *convstore.User:
			end(agent.EndInterrupted, nil)
			turn++
			running = true
			play(&agent.TurnStarted{Turn: turn, Text: r.Text})
		case *convstore.Assistant:
			replayReply(play, r, cur)
			switch {
			case r.CutOff:
				end(agent.EndInterrupted, nil)
			case r.Stop != convstore.StopToolUse:
				end(agent.EndDone, nil)
			}
		case *convstore.ToolResult:
			play(&agent.ToolResult{ID: r.ToolUseID, Content: r.Content, IsError: r.IsError})
		case *convstore.Interruption:
			if r.Failed {
				end(agent.EndFailed, &agent.Failure{Kind: agent.FailOther, Message: strings.TrimPrefix(r.Reason, "failed: ")})
			} else {
				end(agent.EndInterrupted, nil)
			}
		case *convstore.Question:
			play(&agent.Question{ID: r.ToolUseID, Text: r.Text, Multi: r.Multi, Options: replayOptions(r.Options)})
		case *convstore.Answer:
			play(&agent.QuestionAnswered{ID: r.ToolUseID})
		case *convstore.ModelSwitch:
			cur = r.Model
			play(&agent.ModelChanged{Model: r.Model, Effort: r.Effort})
		}
	}
	end(agent.EndInterrupted, nil)
	snap.Title = loaded.Title()
	return snap
}

func replayOptions(opts []convstore.QuestionOption) []agent.Option {
	out := make([]agent.Option, len(opts))
	for i, o := range opts {
		out[i] = agent.Option{Label: o.Label, Description: o.Description, Preview: o.Preview}
	}
	return out
}

// replayReply plays the blocks of one saved reply, then its usage.
func replayReply(play func(agent.Body), r *convstore.Assistant, model string) {
	for _, b := range r.Blocks {
		switch b.Kind {
		case convstore.BlockText:
			play(&agent.TextDelta{Text: b.Text})
		case convstore.BlockThinking:
			play(&agent.ThinkingDelta{Text: b.Text})
		case convstore.BlockToolUse:
			play(&agent.ToolCall{ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}
	if r.Usage == nil {
		return
	}
	u := &agent.Usage{
		Requested: model, Reported: r.Model, Input: r.Usage.Input, Output: r.Usage.Output,
		CacheRead: r.Usage.CacheRead, CacheWrite: r.Usage.CacheWrite,
	}
	if name, err := provider.ParseModelName(model); err == nil {
		u.Cost, u.Priced = provider.Cost(name, provider.Usage{
			Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite,
		})
	}
	play(u)
}
