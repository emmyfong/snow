package openai

import (
	"context"
	"io"
	"slices"
	"strconv"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/emmyfong/snow/internal/provider"
)

// chatStream turns a Chat Completions stream into provider events. Text
// goes out as it arrives. A tool call's name and arguments come in pieces,
// so the calls wait until the reply ends, then go out whole. Usage arrives
// in a chunk after finish_reason, so Stop waits for the end of the stream.
type chatStream struct {
	ctx    context.Context
	sse    *ssestream.Stream[sdk.ChatCompletionChunk]
	queue  []provider.Event
	done   bool  // every event is queued
	err    error // the error that ended the reply; Next returns it again
	closed bool
	key    string // the key the request sent, which errors must not repeat

	started bool
	finish  string // finish_reason, once the model has stopped
	refused bool   // the reply holds a refusal
	calls   map[int64]*chatCall
	usage   *provider.Usage
}

// chatCall is a tool call being joined from its pieces.
type chatCall struct {
	id, name string
	args     []byte
}

func newChatStream(ctx context.Context, sse *ssestream.Stream[sdk.ChatCompletionChunk], key string) *chatStream {
	return &chatStream{ctx: ctx, sse: sse, key: key, calls: map[int64]*chatCall{}}
}

// Next implements provider.Stream.
func (s *chatStream) Next() (provider.Event, error) {
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
				return nil, s.err
			}
			if s.finish == "" {
				// The stream ended before finish_reason: the reply was cut.
				s.err = &provider.Error{Kind: provider.KindNetwork, Err: io.ErrUnexpectedEOF}
				return nil, s.err
			}
			s.end()
			continue
		}
		s.handle(s.sse.Current())
	}
}

// Close implements provider.Stream.
func (s *chatStream) Close() error {
	s.closed = true
	return s.sse.Close()
}

func (s *chatStream) handle(c sdk.ChatCompletionChunk) {
	if !s.started {
		s.started = true
		s.queue = append(s.queue, &provider.Start{Model: c.Model})
	}
	if u := c.Usage; u.PromptTokens > 0 || u.CompletionTokens > 0 || u.TotalTokens > 0 {
		cached, written := int(u.PromptTokensDetails.CachedTokens), int(u.PromptTokensDetails.CacheWriteTokens)
		s.usage = &provider.Usage{
			Input:      max(int(u.PromptTokens)-cached-written, 0),
			Output:     int(u.CompletionTokens),
			CacheRead:  cached,
			CacheWrite: written,
		}
	}
	// One choice is asked for. A service that sends more has the first one
	// read and the rest ignored.
	for _, ch := range c.Choices {
		if ch.Index != 0 {
			continue
		}
		if ch.Delta.Content != "" {
			s.queue = append(s.queue, &provider.TextDelta{Text: ch.Delta.Content})
		}
		if ch.Delta.Refusal != "" {
			s.refused = true
			s.queue = append(s.queue, &provider.TextDelta{Text: ch.Delta.Refusal})
		}
		for _, tc := range ch.Delta.ToolCalls {
			call := s.calls[tc.Index]
			if call == nil {
				call = &chatCall{}
				s.calls[tc.Index] = call
			}
			if tc.ID != "" {
				call.id = tc.ID
			}
			if tc.Function.Name != "" {
				call.name = tc.Function.Name
			}
			call.args = append(call.args, tc.Function.Arguments...)
		}
		if ch.FinishReason != "" {
			s.finish = ch.FinishReason
		}
	}
}

// end queues what waited for the reply to finish: the tool calls in order
// of their index, the usage, and the stop reason.
func (s *chatStream) end() {
	indexes := make([]int64, 0, len(s.calls))
	for i := range s.calls {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)
	for _, i := range indexes {
		call := s.calls[i]
		id := call.id
		if id == "" {
			// A service that sends no ID still needs one to pair the result.
			id = "call_" + strconv.FormatInt(i, 10)
		}
		args := call.args
		if len(args) == 0 {
			args = []byte("{}")
		}
		s.queue = append(s.queue, &provider.ToolUse{ID: id, Name: call.name, Input: args})
	}
	if s.usage != nil {
		s.queue = append(s.queue, s.usage)
	}
	s.queue = append(s.queue, &provider.Stop{Reason: s.stopReason(len(indexes) > 0)})
	s.done = true
}

// stopReason maps finish_reason. A reply that holds a tool call stops for
// the tool whatever the service says: Ollama reports "stop" for it.
func (s *chatStream) stopReason(tools bool) provider.StopReason {
	switch {
	case tools:
		return provider.StopToolUse
	case s.refused || s.finish == "content_filter":
		return provider.StopRefusal
	case s.finish == "stop":
		return provider.StopEndTurn
	case s.finish == "length":
		return provider.StopMaxTokens
	}
	return provider.StopOther
}
