package openai

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
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
	finish  string              // finish_reason, once the model has stopped
	refused bool                // the reply holds a refusal
	calls   []*chatCall         // in order of arrival
	open    map[int64]*chatCall // the call each index adds to now
	usage   *provider.Usage
	prefix  string // makes the ids Snow gives to calls unique to this reply
}

// chatCall is a tool call being joined from its pieces.
type chatCall struct {
	index    int64
	id, name string
	args     []byte
}

func newChatStream(ctx context.Context, sse *ssestream.Stream[sdk.ChatCompletionChunk], key string) *chatStream {
	return &chatStream{ctx: ctx, sse: sse, key: key, open: map[int64]*chatCall{}}
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
	if s.prefix == "" && c.ID != "" {
		s.prefix = "call_" + c.ID + "_"
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
			call := s.open[tc.Index]
			if call != nil && tc.ID != "" && call.id != "" && tc.ID != call.id {
				// A new id on an index in use starts a new call: some services
				// send no index, or the same one for every call.
				call = nil
			}
			if call == nil {
				call = &chatCall{index: tc.Index}
				s.open[tc.Index] = call
				s.calls = append(s.calls, call)
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
	slices.SortStableFunc(s.calls, func(a, b *chatCall) int { return cmp.Compare(a.index, b.index) })
	for _, call := range s.calls {
		id := call.id
		if id == "" {
			// A service that sends no ID still needs one to pair the result.
			id = s.callID(call.index)
		}
		s.queue = append(s.queue, &provider.ToolUse{ID: id, Name: call.name, Input: validArguments(call.args)})
	}
	if s.usage != nil {
		s.queue = append(s.queue, s.usage)
	}
	s.queue = append(s.queue, &provider.Stop{Reason: s.stopReason(len(s.calls) > 0)})
	s.done = true
}

// callID makes an ID for the call at index. It holds the reply's own ID, or
// a random one when the service sends none, so no two replies share an ID
// and a later result cannot pair with an old call.
func (s *chatStream) callID(index int64) string {
	if s.prefix == "" {
		s.prefix = "call_" + rand.Text()[:8] + "_"
	}
	return s.prefix + strconv.FormatInt(index, 10)
}

// validArguments returns args as JSON the conversation file can hold. A
// model, a small local one above all, can write arguments that are not
// JSON, and a reply cut by the token limit ends in the middle of them. Such
// text goes on as a JSON string: no tool takes a string, so the tool layer
// answers the model with an error that it can act on.
func validArguments(args []byte) json.RawMessage {
	if len(args) == 0 {
		return json.RawMessage("{}")
	}
	if json.Valid(args) {
		return args
	}
	quoted, _ := json.Marshal(string(args)) // a string always marshals
	return quoted
}

// stopReason maps finish_reason. A reply that holds a tool call stops for
// the tool whatever the service says, because Ollama reports "stop" for
// it. A reply cut off or filtered says so first: the agent does not run the
// calls of a reply that did not finish.
func (s *chatStream) stopReason(tools bool) provider.StopReason {
	switch {
	case s.finish == "length":
		return provider.StopMaxTokens
	case s.refused || s.finish == "content_filter":
		return provider.StopRefusal
	case tools:
		return provider.StopToolUse
	case s.finish == "stop":
		return provider.StopEndTurn
	}
	return provider.StopOther
}
