package openai

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"

	"github.com/emmyfong/snow/internal/provider"
)

// stream turns the SDK's server-sent events into provider events. One SDK
// event can give zero, one, or two provider events, so they queue.
type stream struct {
	ctx    context.Context
	sse    *ssestream.Stream[responses.ResponseStreamEventUnion]
	queue  []provider.Event
	done   bool  // the response completed or stopped early
	err    error // the error that ended the reply; Next returns it again
	closed bool
	key    string // the key the request sent, which errors must not repeat

	toolCalls bool   // the reply holds a tool call
	refused   bool   // the reply holds a refusal
	lastItem  string // the reasoning item whose summary streamed last
}

func newStream(ctx context.Context, sse *ssestream.Stream[responses.ResponseStreamEventUnion], key string) *stream {
	return &stream{ctx: ctx, sse: sse, key: key}
}

// Next implements provider.Stream.
func (s *stream) Next() (provider.Event, error) {
	for {
		if s.closed {
			return nil, io.EOF
		}
		if s.err != nil {
			return nil, s.err
		}
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
				s.err = mapErr(s.ctx, err, s.key)
			} else {
				// The connection ended before response.completed: the reply was cut.
				s.err = &provider.Error{Kind: provider.KindNetwork, Err: io.ErrUnexpectedEOF}
			}
			return nil, s.err
		}
		if err := s.handle(s.sse.Current()); err != nil {
			s.err = err
			return nil, err
		}
	}
}

// Close implements provider.Stream.
func (s *stream) Close() error {
	s.closed = true
	return s.sse.Close()
}

func (s *stream) handle(ev responses.ResponseStreamEventUnion) error {
	switch e := ev.AsAny().(type) {
	case responses.ResponseCreatedEvent:
		s.queue = append(s.queue, &provider.Start{Model: e.Response.Model})
	case responses.ResponseTextDeltaEvent:
		s.queue = append(s.queue, &provider.TextDelta{Text: e.Delta})
	case responses.ResponseRefusalDeltaEvent:
		s.refused = true
		s.queue = append(s.queue, &provider.TextDelta{Text: e.Delta})
	case responses.ResponseReasoningSummaryPartAddedEvent:
		if e.SummaryIndex > 0 || (s.lastItem != "" && e.ItemID != s.lastItem) {
			s.queue = append(s.queue, &provider.ThinkingDelta{Text: summarySep})
		}
		s.lastItem = e.ItemID
	case responses.ResponseReasoningSummaryTextDeltaEvent:
		s.queue = append(s.queue, &provider.ThinkingDelta{Text: e.Delta})
	case responses.ResponseOutputItemDoneEvent:
		s.itemDone(e.Item)
	case responses.ResponseCompletedEvent:
		reason := provider.StopEndTurn
		switch {
		case s.toolCalls:
			reason = provider.StopToolUse
		case s.refused:
			reason = provider.StopRefusal
		}
		s.finish(e.Response.Usage, reason)
	case responses.ResponseIncompleteEvent:
		s.finish(e.Response.Usage, incompleteReason(e.Response.IncompleteDetails.Reason))
	case responses.ResponseFailedEvent:
		er := e.Response.Error
		return failure(0, string(er.Code), er.Type, er.Message, s.key)
	case responses.ResponseErrorEvent:
		return failure(0, e.Code, "", e.Message, s.key)
	}
	return nil
}

// summarySep joins the parts of one reasoning summary.
const summarySep = "\n\n"

func (s *stream) itemDone(item responses.ResponseOutputItemUnion) {
	switch it := item.AsAny().(type) {
	case responses.ResponseReasoningItem:
		// Without its encrypted content, a reasoning item cannot be sent
		// back when nothing is stored, so there is nothing to keep.
		if it.EncryptedContent == "" {
			return
		}
		ref := reasoningRef{ID: it.ID, Encrypted: it.EncryptedContent}.encode()
		parts := make([]string, len(it.Summary))
		for i, p := range it.Summary {
			parts[i] = p.Text
		}
		if text := strings.Join(parts, summarySep); text != "" {
			s.queue = append(s.queue, &provider.Thinking{Text: text, Signature: ref})
		} else {
			s.queue = append(s.queue, &provider.RedactedThinking{Data: ref})
		}
	case responses.ResponseFunctionToolCall:
		s.toolCalls = true
		input := json.RawMessage(it.Arguments)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		s.queue = append(s.queue, &provider.ToolUse{ID: it.CallID, Name: it.Name, Input: input})
	}
}

// finish ends the reply with its usage and reason.
func (s *stream) finish(u responses.ResponseUsage, reason provider.StopReason) {
	usage := toUsage(u)
	s.queue = append(s.queue, &usage, &provider.Stop{Reason: reason})
	s.done = true
}

// toUsage converts the API's counts. Its input total includes cached and
// cache-written tokens; provider.Usage keeps them apart.
func toUsage(u responses.ResponseUsage) provider.Usage {
	cached, written := int(u.InputTokensDetails.CachedTokens), int(u.InputTokensDetails.CacheWriteTokens)
	return provider.Usage{
		Input:      max(int(u.InputTokens)-cached-written, 0),
		Output:     int(u.OutputTokens),
		CacheRead:  cached,
		CacheWrite: written,
	}
}

func incompleteReason(r string) provider.StopReason {
	switch r {
	case "max_output_tokens":
		return provider.StopMaxTokens
	case "content_filter":
		return provider.StopRefusal
	default:
		return provider.StopOther
	}
}
