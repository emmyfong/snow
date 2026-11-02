package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// TestChatStreamText: text deltas, usage, and the stop reason come from a
// Chat Completions stream, and Stop comes last, after the usage chunk.
func TestChatStreamText(t *testing.T) {
	cs := newChatServer(t, "chat_text.sse")
	evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "llama3.1:8b", Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	want := []provider.Event{
		&provider.Start{Model: "llama3.1:8b"},
		&provider.TextDelta{Text: "Hel"},
		&provider.TextDelta{Text: "lo"},
		&provider.Usage{Input: 8, Output: 3, CacheRead: 4},
		&provider.Stop{Reason: provider.StopEndTurn},
	}
	if !reflect.DeepEqual(evs, want) {
		t.Fatalf("events\n got %s\nwant %s", dump(evs), dump(want))
	}
}

// TestChatStreamToolCall: two tool calls whose arguments arrive in several
// chunks, mixed together, come out whole and in order of their index.
func TestChatStreamToolCall(t *testing.T) {
	cs := newChatServer(t, "chat_tool_calls.sse")
	evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	var uses []*provider.ToolUse
	for _, e := range evs {
		if u, ok := e.(*provider.ToolUse); ok {
			uses = append(uses, u)
		}
	}
	if len(uses) != 2 ||
		uses[0].ID != "call_a" || uses[0].Name != "read_file" || string(uses[0].Input) != `{"path":"a.go"}` ||
		uses[1].ID != "call_b" || uses[1].Name != "run" || string(uses[1].Input) != `{"cmd":"ls"}` {
		t.Fatalf("tool uses %+v", uses)
	}
	last := evs[len(evs)-1]
	if stop, ok := last.(*provider.Stop); !ok || stop.Reason != provider.StopToolUse {
		t.Fatalf("last event %#v, want Stop(tool use)", last)
	}
	if u := providertest.Only[*provider.Usage](evs); len(u) != 1 || u[0].Input != 20 || u[0].Output != 9 {
		t.Fatalf("usage %+v", u)
	}
}

// TestChatStopReasons: the stop reason follows finish_reason, and a reply
// that holds a tool call stops for the tool even when the service says
// "stop", as Ollama does.
func TestChatStopReasons(t *testing.T) {
	for _, tt := range []struct {
		fixture string
		want    provider.StopReason
	}{
		{"chat_text.sse", provider.StopEndTurn},
		{"chat_tool_calls.sse", provider.StopToolUse},
		{"chat_tool_stop.sse", provider.StopToolUse},
		{"chat_length.sse", provider.StopMaxTokens},
		// A reply cut by the token limit is cut, whatever it holds: the agent
		// answers its calls as lost instead of running them.
		{"chat_length_tool.sse", provider.StopMaxTokens},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			cs := newChatServer(t, tt.fixture)
			evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
			if err != nil {
				t.Fatal(err)
			}
			stops := providertest.Only[*provider.Stop](evs)
			if len(stops) != 1 || stops[0].Reason != tt.want {
				t.Fatalf("stops %+v, want %v", stops, tt.want)
			}
		})
	}
}

// TestChatBadArguments: arguments that are not JSON, because the model
// wrote them badly or the reply was cut, still reach the agent as JSON. A
// conversation file holds only valid JSON, and the tool layer then answers
// the model with an error it can act on (#225).
func TestChatBadArguments(t *testing.T) {
	for _, tt := range []struct {
		fixture string
		stop    provider.StopReason
		inputs  []string // each Input as the agent receives it
	}{
		{"chat_bad_args.sse", provider.StopToolUse, []string{`"{\"path\":"`}},
		{"chat_length_tool.sse", provider.StopMaxTokens, []string{`{"path":"a.go"}`, `"{\"cmd\":\"l"`}},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			cs := newChatServer(t, tt.fixture)
			evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, u := range providertest.Only[*provider.ToolUse](evs) {
				if !json.Valid(u.Input) {
					t.Fatalf("input %q is not JSON", u.Input)
				}
				got = append(got, string(u.Input))
			}
			if !reflect.DeepEqual(got, tt.inputs) {
				t.Fatalf("inputs %q, want %q", got, tt.inputs)
			}
			if stops := providertest.Only[*provider.Stop](evs); len(stops) != 1 || stops[0].Reason != tt.stop {
				t.Fatalf("stops %+v, want %v", stops, tt.stop)
			}
		})
	}
}

// TestChatToolCallsWithoutIndex: a service that sends no index, or the same
// one for every call, still gets its calls told apart by their ids.
func TestChatToolCallsWithoutIndex(t *testing.T) {
	cs := newChatServer(t, "chat_no_index.sse")
	evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range providertest.Only[*provider.ToolUse](evs) {
		got = append(got, fmt.Sprintf("%s %s %s", u.ID, u.Name, u.Input))
	}
	want := []string{`call_a read_file {"path":"a.go"}`, `call_b run {"cmd":"ls"}`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls %q, want %q", got, want)
	}
}

// TestChatToolCallIDs: a call the service gave no id gets one that pairs
// its result, and no two replies share one.
func TestChatToolCallIDs(t *testing.T) {
	ids := func(fixture string) []string {
		cs := newChatServer(t, fixture)
		evs, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, u := range providertest.Only[*provider.ToolUse](evs) {
			out = append(out, u.ID)
		}
		return out
	}
	if got, want := ids("chat_no_id.sse"), []string{"call_chatcmpl-77_0", "call_chatcmpl-77_1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids %q, want %q", got, want)
	}
	// With no response id either, the ids still differ from reply to reply.
	a, b := ids("chat_no_id_no_response.sse"), ids("chat_no_id_no_response.sse")
	if len(a) != 2 || a[0] == a[1] || a[0] == b[0] || !strings.HasPrefix(a[0], "call_") {
		t.Fatalf("ids %q and %q", a, b)
	}
}

// TestChatCutStream: a stream that ends before finish_reason is a cut
// reply, not a finished one.
func TestChatCutStream(t *testing.T) {
	cs := newChatServer(t, "chat_cut.sse")
	_, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

// TestChatStreamShapes: replies that services send in their own way, each
// still one Usage, one Stop, and the text and calls once.
func TestChatStreamShapes(t *testing.T) {
	text := func(s, finish string) string {
		return sseChunk(`[{"index":0,"delta":{"content":"`+s+`"},"finish_reason":`+finish+`}]`, `"usage":{"prompt_tokens":5,"completion_tokens":2}`)
	}
	for _, tt := range []struct {
		name  string
		body  string
		text  string
		usage []provider.Usage
		stop  provider.StopReason
	}{
		{"usage in every chunk", text("a", "null") + text("b", "null") + text("", `"stop"`) + "data: [DONE]\n\n", "ab", []provider.Usage{{Input: 5, Output: 2}}, provider.StopEndTurn},
		{"no usage at all", sseChunk(`[{"index":0,"delta":{"content":"a"},"finish_reason":"stop"}]`, "") + "data: [DONE]\n\n", "a", nil, provider.StopEndTurn},
		{"content filter", sseChunk(`[{"index":0,"delta":{"content":"a"},"finish_reason":"content_filter"}]`, "") + "data: [DONE]\n\n", "a", nil, provider.StopRefusal},
		{"refusal text", sseChunk(`[{"index":0,"delta":{"refusal":"No."},"finish_reason":"stop"}]`, "") + "data: [DONE]\n\n", "No.", nil, provider.StopRefusal},
		{"unknown reason", sseChunk(`[{"index":0,"delta":{"content":"a"},"finish_reason":"function_call"}]`, "") + "data: [DONE]\n\n", "a", nil, provider.StopOther},
		{"second choice ignored", sseChunk(`[{"index":0,"delta":{"content":"a"},"finish_reason":"stop"},{"index":1,"delta":{"content":"z"},"finish_reason":"stop"}]`, "") + "data: [DONE]\n\n", "a", nil, provider.StopEndTurn},
		{"empty choices first", sseChunk(`[]`, "") + sseChunk(`[{"index":0,"delta":{"content":"a"},"finish_reason":"stop"}]`, "") + "data: [DONE]\n\n", "a", nil, provider.StopEndTurn},
	} {
		t.Run(tt.name, func(t *testing.T) {
			evs := inlineChat(t, tt.body)
			var got strings.Builder
			for _, d := range providertest.Only[*provider.TextDelta](evs) {
				got.WriteString(d.Text)
			}
			var usage []provider.Usage
			for _, u := range providertest.Only[*provider.Usage](evs) {
				usage = append(usage, *u)
			}
			if got.String() != tt.text || !reflect.DeepEqual(usage, tt.usage) {
				t.Fatalf("text %q usage %+v, want %q %+v", got.String(), usage, tt.text, tt.usage)
			}
			if stops := providertest.Only[*provider.Stop](evs); len(stops) != 1 || stops[0].Reason != tt.stop || evs[len(evs)-1] != provider.Event(stops[0]) {
				t.Fatalf("stops %+v, want %v last", stops, tt.stop)
			}
		})
	}
}

// TestChatToolCallWithoutName: a call that arrives with no name still comes
// out whole, with an id, and the tool layer answers that no such tool
// exists.
func TestChatToolCallWithoutName(t *testing.T) {
	evs := inlineChat(t, sseChunk(`[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]},"finish_reason":"tool_calls"}]`, "")+"data: [DONE]\n\n")
	uses := providertest.Only[*provider.ToolUse](evs)
	if len(uses) != 1 || uses[0].Name != "" || uses[0].ID != "call_chatcmpl-1_0" || string(uses[0].Input) != "{}" {
		t.Fatalf("tool uses %+v", uses)
	}
}
