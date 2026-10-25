package agent

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/tool"
)

// reply is one whole reply of the model, as it streamed.
type reply struct {
	blocks   []provider.Block
	stop     provider.StopReason
	usage    provider.Usage
	reported string
}

// calls returns the tool calls of the reply.
func (r reply) calls() []*provider.ToolUse {
	var out []*provider.ToolUse
	for _, b := range r.blocks {
		if u, ok := b.(*provider.ToolUse); ok {
			out = append(out, u)
		}
	}
	return out
}

// runTurn runs one turn from the user's message until the model ends it,
// the user interrupts it, or it fails. Every record is saved before the
// next step depends on it.
func (c *Conversation) runTurn(ctx context.Context, n int, text string) {
	c.emit(&TurnStarted{Turn: n})
	if err := c.w.Append(&convstore.User{At: c.now(), Text: text}); err != nil {
		// Nothing goes to the provider that could not be saved.
		c.end(n, EndFailed, &Failure{Kind: FailSave, Message: err.Error()})
		return
	}
	c.history = appendMessage(c.history, provider.RoleUser, &provider.Text{Text: text})
	for {
		c.emit(&StateChanged{State: StateStreaming})
		r, err := c.stream(ctx)
		if err != nil {
			c.stop(ctx, n, err)
			return
		}
		if err := c.w.Append(assistantRecord(c.now(), r)); err != nil {
			c.end(n, EndFailed, &Failure{Kind: FailSave, Message: err.Error()})
			return
		}
		c.history = appendMessage(c.history, provider.RoleAssistant, r.blocks...)
		calls := r.calls()
		if r.stop != provider.StopToolUse || len(calls) == 0 {
			if len(calls) > 0 {
				// A reply cut off by its token limit can still hold whole
				// calls that will not run. The file answers them as lost.
				c.resync()
			}
			c.end(n, EndDone, nil)
			return
		}
		results, err := c.runTools(ctx, calls)
		if err != nil {
			c.stop(ctx, n, err)
			return
		}
		c.history = appendMessage(c.history, provider.RoleUser, results...)
	}
}

// stop ends a turn that err cut short. An interrupt and a failure both
// save an Interruption record, after the partial text that waits, so the
// file shows where the turn ended.
func (c *Conversation) stop(ctx context.Context, n int, err error) {
	var we *convstore.WriteError
	if errors.As(err, &we) {
		c.end(n, EndFailed, &Failure{Kind: FailSave, Message: err.Error()})
		return
	}
	reason, end, f := "interrupted by the user", EndInterrupted, (*Failure)(nil)
	if ctx.Err() == nil {
		f = failure(err)
		reason, end = "failed: "+f.Message, EndFailed
	}
	if err := c.w.Append(&convstore.Interruption{At: c.now(), Reason: reason}); err != nil {
		c.end(n, EndFailed, &Failure{Kind: FailSave, Message: err.Error()})
		return
	}
	c.end(n, end, f)
}

// end closes the turn. A turn that did not finish leaves the live history
// short of what the file holds: calls with no results, or text that was
// cut off. The history is then rebuilt from the file, so the next request
// is the one a resume would send.
func (c *Conversation) end(n int, reason EndReason, f *Failure) {
	if reason != EndDone {
		c.resync()
	}
	c.emit(&TurnEnded{Turn: n, Reason: reason, Failure: f})
}

// resync rebuilds the live history from the conversation file. If the
// file cannot be read, the history stays as it is; the next request then
// fails with a provider error the user can act on.
func (c *Conversation) resync() {
	loaded, err := c.cfg.Store.Load(c.id)
	if err != nil {
		return
	}
	c.history = history(loaded.Records)
}

// stream sends the conversation to the model and collects the reply. Text
// goes out as events and is saved as partial text while it streams.
func (c *Conversation) stream(ctx context.Context) (reply, error) {
	name, err := provider.ParseModelName(c.model)
	if err != nil {
		return reply{}, err
	}
	p, ok := c.cfg.Providers.Get(name.Provider)
	if !ok {
		return reply{}, &provider.Error{Kind: provider.KindNoKey, Err: errors.New("provider " + name.Provider + " is not connected")}
	}
	req := provider.Request{Model: name.Model, Effort: c.effort, System: c.cfg.System, Messages: c.history, Tools: c.toolDefs()}
	s, err := p.Stream(ctx, req)
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = s.Close() }()
	var (
		r    reply
		text strings.Builder // text of the current text block
		all  strings.Builder // all the reply's text
	)
	flush := func() {
		if text.Len() > 0 {
			r.blocks = append(r.blocks, &provider.Text{Text: text.String()})
			text.Reset()
		}
	}
	for {
		ev, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return reply{}, err
		}
		switch e := ev.(type) {
		case *provider.Start:
			r.reported = e.Model
		case *provider.TextDelta:
			text.WriteString(e.Text)
			all.WriteString(e.Text)
			c.emit(&TextDelta{Text: e.Text})
			if err := c.w.AppendPartial(convstore.Partial{At: c.now(), Text: all.String()}); err != nil {
				return reply{}, err
			}
		case *provider.ThinkingDelta:
			c.emit(&ThinkingDelta{Text: e.Text})
		case *provider.Thinking:
			flush()
			r.blocks = append(r.blocks, e)
		case *provider.RedactedThinking:
			flush()
			r.blocks = append(r.blocks, e)
		case *provider.ToolUse:
			flush()
			r.blocks = append(r.blocks, e)
			c.emit(&ToolCall{ID: e.ID, Name: e.Name, Input: e.Input})
		case *provider.Usage:
			r.usage = *e
		case *provider.Stop:
			r.stop = e.Reason
		}
	}
	flush()
	c.emitUsage(name, r)
	return r, nil
}

func (c *Conversation) emitUsage(name provider.ModelName, r reply) {
	u := &Usage{Requested: name.String(), Reported: r.reported,
		Input: r.usage.Input, Output: r.usage.Output, CacheRead: r.usage.CacheRead, CacheWrite: r.usage.CacheWrite}
	u.Cost, u.Priced = provider.Cost(name, r.usage)
	c.emit(u)
}

func (c *Conversation) toolDefs() []provider.ToolDef {
	var defs []provider.ToolDef
	for _, d := range c.cfg.Tools.Defs() {
		defs = append(defs, provider.ToolDef{Name: d.Name, Description: d.Description, Schema: d.Schema})
	}
	return defs
}

// runTools runs the calls of a reply in order and saves each result. After
// an interrupt, the calls that did not finish get an "interrupted" result,
// so the saved exchange stays valid for the provider.
func (c *Conversation) runTools(ctx context.Context, calls []*provider.ToolUse) ([]provider.Block, error) {
	env := c.env()
	var results []provider.Block
	for _, call := range calls {
		var res tool.Result
		started := ctx.Err() == nil
		if started {
			c.emit(&StateChanged{State: StateTool})
			res = c.cfg.Tools.Call(ctx, env, call.Name, call.Input, c.approver(call.ID))
		}
		// A call that finished despite the interrupt keeps its result: a
		// write that happened must be reported as done. One that never
		// started, or stopped with an error, was interrupted.
		if ctx.Err() != nil && (!started || res.IsError) {
			res = tool.Result{Content: "interrupted by the user", IsError: true}
		}
		if err := c.w.Append(&convstore.ToolResult{At: c.now(), ToolUseID: call.ID, Content: res.Content, IsError: res.IsError}); err != nil {
			return nil, err
		}
		c.emit(&ToolResult{ID: call.ID, Content: res.Content, IsError: res.IsError})
		results = append(results, &provider.ToolResult{ToolUseID: call.ID, Content: res.Content, IsError: res.IsError})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
