package server

import (
	"encoding/json"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// Bounds on what one chat message carries, so every event and snapshot
// fits a frame. The model gets the full text; the chat shows a capped
// view.
const (
	maxDiffText   = 128 << 10 // each side of a diff
	maxInputValue = 4 << 10   // one string value in a tool call's input
	// eventBudget bounds one encoded event. Text that JSON escapes to six
	// bytes per byte passes the caps above, so an event is measured too.
	eventBudget = api.MaxFrameSize / 3
)

// chatEvent turns an agent event for chat pane id into its protocol form,
// or nil for a kind the protocol does not carry yet.
func chatEvent(id int, e agent.Event) *api.ChatEvent {
	out := &api.ChatEvent{Pane: id, Agent: string(e.Agent)}
	switch b := e.Body.(type) {
	case *agent.TurnStarted:
		out.Kind, out.Turn, out.Text = api.EventTurnStarted, b.Turn, b.Text
	case *agent.TurnEnded:
		out.Kind, out.Turn, out.Reason, out.Error = api.EventTurnEnded, b.Turn, string(b.Reason), chatError(b.Failure)
	case *agent.TextDelta:
		out.Kind, out.Text = api.EventText, b.Text
	case *agent.ThinkingDelta:
		out.Kind, out.Text = api.EventThinking, b.Text
	case *agent.ToolCall:
		out.Kind, out.Tool = api.EventToolCall, &api.ChatTool{ID: b.ID, Name: b.Name, Input: capInput(b.Input)}
	case *agent.ToolResult:
		out.Kind = api.EventToolResult
		out.Tool = &api.ChatTool{ID: b.ID, Done: true, Result: capText(b.Content, api.MaxToolResult), IsError: b.IsError}
	case *agent.ApprovalRequest:
		out.Kind = api.EventApproval
		out.Approval = &api.ChatApproval{ID: b.ID, Tool: b.Tool, Input: capInput(b.Input), Diff: chatDiff(b.Diff), Outside: b.Outside}
	case *agent.StateChanged:
		out.Kind, out.State = api.EventState, string(b.State)
	case *agent.QueueChanged:
		out.Kind, out.Queue = api.EventQueue, b.Messages
	case *agent.Usage:
		out.Kind = api.EventUsage
		out.Usage = &api.ChatUsage{
			Requested: b.Requested, Reported: b.Reported, Input: b.Input, Output: b.Output,
			CacheRead: b.CacheRead, CacheWrite: b.CacheWrite, Cost: b.Cost, Priced: b.Priced,
		}
		if name, err := provider.ParseModelName(b.Requested); err == nil {
			out.Usage.Window = provider.ContextWindow(name, 0)
		}
	case *agent.ModelChanged:
		out.Kind, out.Model, out.Effort, out.Error = api.EventModel, b.Model, b.Effort, chatError(b.Failure)
	case *agent.ModeChanged:
		out.Kind, out.Mode, out.Error = api.EventMode, string(b.Mode), chatError(b.Failure)
	default:
		return nil
	}
	fitEvent(out)
	return out
}

// fitEvent cuts e's tool input and diff further until its encoding fits
// eventBudget, and drops them if even the smallest cut does not fit.
func fitEvent(e *api.ChatEvent) {
	if e.Tool == nil && e.Approval == nil {
		return
	}
	for n := maxDiffText / 4; ; n /= 4 {
		if b, err := json.Marshal(e); err == nil && len(b) <= eventBudget {
			return
		}
		if n < 1<<10 {
			break
		}
		if t := e.Tool; t != nil {
			t.Input = capInputTo(t.Input, n)
		}
		if a := e.Approval; a != nil {
			a.Input = capInputTo(a.Input, n)
			if a.Diff != nil {
				a.Diff.Old, a.Diff.New = capText(a.Diff.Old, n), capText(a.Diff.New, n)
			}
		}
	}
	if t := e.Tool; t != nil {
		t.Input = nil
	}
	if a := e.Approval; a != nil {
		a.Input, a.Diff = nil, nil
	}
}

// errorCodes maps agent failure kinds to protocol codes.
var errorCodes = map[agent.FailureKind]string{
	agent.FailSave: api.ChatErrSave, agent.FailNoKey: api.ChatErrNoKey, agent.FailBadKey: api.ChatErrBadKey,
	agent.FailModelNotFound: api.ChatErrModelNotFound, agent.FailRateLimited: api.ChatErrRateLimited,
	agent.FailOverloaded: api.ChatErrOverloaded, agent.FailContextTooLong: api.ChatErrContextTooLong,
	agent.FailNetwork: api.ChatErrNetwork, agent.FailOther: api.ChatErrOther,
}

// choices maps agent choices to protocol choices.
var choices = map[agent.Choice]string{
	agent.ChoiceRetry: api.ChatChoiceRetry, agent.ChoicePickModel: api.ChatChoicePickModel,
	agent.ChoiceWait: api.ChatChoiceWait, agent.ChoiceLogin: api.ChatChoiceLogin,
}

func chatError(f *agent.Failure) *api.ChatError {
	if f == nil {
		return nil
	}
	code, ok := errorCodes[f.Kind]
	if !ok {
		code = api.ChatErrOther
	}
	e := &api.ChatError{Code: code, Message: f.Message, RetryAfter: f.RetryAfter.Seconds()}
	for _, c := range f.Choices {
		if s, ok := choices[c]; ok {
			e.Choices = append(e.Choices, s)
		}
	}
	return e
}

func chatDiff(d *agent.Diff) *api.ChatDiff {
	if d == nil {
		return nil
	}
	return &api.ChatDiff{Path: d.Path, Old: capText(d.Old, maxDiffText), New: capText(d.New, maxDiffText)}
}

// capText returns s cut to at most n bytes, on a character boundary, with
// a note that it was cut.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	const note = "\n… (cut)"
	i := n - len(note)
	for i > 0 && !utf8Start(s[i]) {
		i--
	}
	return s[:i] + note
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// capInput returns a tool call's input with long string values cut, such
// as the whole file a write sends. Input that is not a JSON object is
// dropped when it is too long to show.
func capInput(raw json.RawMessage) json.RawMessage { return capInputTo(raw, maxInputValue) }

// capInputTo is capInput with string values cut to n bytes.
func capInputTo(raw json.RawMessage, n int) json.RawMessage {
	if len(raw) <= n {
		return raw
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			m[k] = capText(s, n)
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}
