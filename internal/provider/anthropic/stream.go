package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"

	"github.com/emmyfong/snow/internal/provider"
)

// stream turns the SDK's server-sent events into provider events. One SDK
// event can give zero, one, or two provider events, so they queue.
type stream struct {
	ctx   context.Context
	sse   *ssestream.Stream[sdk.MessageStreamEventUnion]
	queue []provider.Event
	usage provider.Usage
	stop  provider.StopReason
	done  bool

	// The block being streamed, kept until its stop event.
	kind      string
	toolID    string
	toolName  string
	toolInput strings.Builder
	thinking  strings.Builder
	signature string
}

func newStream(ctx context.Context, sse *ssestream.Stream[sdk.MessageStreamEventUnion]) *stream {
	return &stream{ctx: ctx, sse: sse}
}

// Next implements provider.Stream.
func (s *stream) Next() (provider.Event, error) {
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		if len(s.queue) > 0 {
			ev := s.queue[0]
			s.queue = s.queue[1:]
			return ev, nil
		}
		if s.done {
			return nil, io.EOF
		}
		if !s.sse.Next() {
			if err := s.sse.Err(); err != nil {
				return nil, mapErr(s.ctx, err)
			}
			// The connection ended without message_stop: the reply was cut.
			return nil, &provider.Error{Kind: provider.KindNetwork, Err: io.ErrUnexpectedEOF}
		}
		s.handle(s.sse.Current())
	}
}

// Close implements provider.Stream.
func (s *stream) Close() error {
	s.done = true
	return s.sse.Close()
}

func (s *stream) handle(ev sdk.MessageStreamEventUnion) {
	switch e := ev.AsAny().(type) {
	case sdk.MessageStartEvent:
		s.usage = provider.Usage{
			Input:      int(e.Message.Usage.InputTokens),
			Output:     int(e.Message.Usage.OutputTokens),
			CacheRead:  int(e.Message.Usage.CacheReadInputTokens),
			CacheWrite: int(e.Message.Usage.CacheCreationInputTokens),
		}
		u := s.usage
		s.queue = append(s.queue, &provider.Start{Model: string(e.Message.Model)}, &u)
	case sdk.ContentBlockStartEvent:
		s.kind = e.ContentBlock.Type
		s.toolInput.Reset()
		s.thinking.Reset()
		s.signature = ""
		if tu, ok := e.ContentBlock.AsAny().(sdk.ToolUseBlock); ok {
			s.toolID, s.toolName = tu.ID, tu.Name
		}
	case sdk.ContentBlockDeltaEvent:
		switch d := e.Delta.AsAny().(type) {
		case sdk.TextDelta:
			s.queue = append(s.queue, &provider.TextDelta{Text: d.Text})
		case sdk.ThinkingDelta:
			s.thinking.WriteString(d.Thinking)
			s.queue = append(s.queue, &provider.ThinkingDelta{Text: d.Thinking})
		case sdk.SignatureDelta:
			s.signature += d.Signature
		case sdk.InputJSONDelta:
			s.toolInput.WriteString(d.PartialJSON)
		}
	case sdk.ContentBlockStopEvent:
		switch s.kind {
		case "tool_use":
			input := json.RawMessage(s.toolInput.String())
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			s.queue = append(s.queue, &provider.ToolUse{ID: s.toolID, Name: s.toolName, Input: input})
		case "thinking":
			s.queue = append(s.queue, &provider.Thinking{Text: s.thinking.String(), Signature: s.signature})
		}
		s.kind = ""
	case sdk.MessageDeltaEvent:
		s.stop = stopReason(e.Delta.StopReason)
		// The delta's counts are totals so far, and a zero means "not sent".
		if e.Usage.InputTokens > 0 {
			s.usage.Input = int(e.Usage.InputTokens)
		}
		if e.Usage.CacheReadInputTokens > 0 {
			s.usage.CacheRead = int(e.Usage.CacheReadInputTokens)
		}
		if e.Usage.CacheCreationInputTokens > 0 {
			s.usage.CacheWrite = int(e.Usage.CacheCreationInputTokens)
		}
		s.usage.Output = int(e.Usage.OutputTokens)
		u := s.usage
		s.queue = append(s.queue, &u)
	case sdk.MessageStopEvent:
		s.queue = append(s.queue, &provider.Stop{Reason: s.stop})
		s.done = true
	}
}

func stopReason(r sdk.StopReason) provider.StopReason {
	switch r {
	case sdk.StopReasonEndTurn:
		return provider.StopEndTurn
	case sdk.StopReasonToolUse:
		return provider.StopToolUse
	case sdk.StopReasonMaxTokens:
		return provider.StopMaxTokens
	case sdk.StopReasonRefusal:
		return provider.StopRefusal
	case sdk.StopReasonModelContextWindowExceeded:
		return provider.StopContext
	default:
		return provider.StopOther
	}
}
