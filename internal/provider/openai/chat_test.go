package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// chatServer is a service that serves the Chat Completions API. It answers
// /chat/completions with a fixture and records each request.
type chatServer struct {
	*httptest.Server
	mu     sync.Mutex
	auths  []string
	paths  []string
	bodies []map[string]any
}

func newChatServer(t *testing.T, fixture string) *chatServer {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	cs := &chatServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		cs.mu.Lock()
		cs.auths = append(cs.auths, r.Header.Get("Authorization"))
		cs.paths = append(cs.paths, r.URL.Path)
		cs.bodies = append(cs.bodies, body)
		cs.mu.Unlock()
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(data)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *chatServer) last() (auth, path string, body map[string]any) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	n := len(cs.paths) - 1
	return cs.auths[n], cs.paths[n], cs.bodies[n]
}

// chatProvider is a provider for a chat service at url.
func chatProvider(url string, noKey bool) *Provider {
	return NewCompatible(Compat{Name: "gw", BaseURL: url + "/v1", NoKey: noKey, Chat: true}, namedKeys{"gw": compatKey})
}

func collectChat(t *testing.T, p *Provider, req provider.Request) ([]provider.Event, error) {
	t.Helper()
	s, err := p.Stream(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return providertest.Collect(t, s)
}

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

// dump writes events for a failure message.
func dump(evs []provider.Event) string {
	var parts []string
	for _, e := range evs {
		parts = append(parts, fmt.Sprintf("%T%+v", e, e))
	}
	return strings.Join(parts, " ")
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

// TestChatRequest: the request goes to /chat/completions with the system
// message, the history with a tool call and its result, the tool
// definitions, a usage request, and no reasoning effort.
func TestChatRequest(t *testing.T) {
	cs := newChatServer(t, "chat_text.sse")
	req := provider.Request{
		Model: "llama3.1:8b", Effort: provider.EffortHigh, System: "You are Snow.", MaxTokens: 512,
		Messages: []provider.Message{
			user("Read a.go"),
			{Role: provider.RoleAssistant, Content: []provider.Block{
				&provider.Thinking{Text: "hmm", Signature: "sig"},
				&provider.Text{Text: "Reading."},
				&provider.ToolUse{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			}},
			{Role: provider.RoleUser, Content: []provider.Block{
				&provider.ToolResult{ToolUseID: "call_1", Content: "package a"},
				&provider.Text{Text: "Now summarize."},
			}},
		},
		Tools: []provider.ToolDef{{Name: "read_file", Description: "Read a file.", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)}},
	}
	if _, err := collectChat(t, chatProvider(cs.URL, false), req); err != nil {
		t.Fatal(err)
	}
	_, path, body := cs.last()
	if path != "/v1/chat/completions" {
		t.Fatalf("path %s", path)
	}
	if body["model"] != "llama3.1:8b" || body["stream"] != true || body["max_tokens"] != float64(512) {
		t.Fatalf("body %v", body)
	}
	for _, k := range []string{"reasoning_effort", "reasoning", "store"} {
		if _, has := body[k]; has {
			t.Errorf("body has %s: %v", k, body[k])
		}
	}
	if so, _ := body["stream_options"].(map[string]any); so["include_usage"] != true {
		t.Errorf("stream_options %v", body["stream_options"])
	}
	msgs, _ := body["messages"].([]any)
	var got []string
	for _, m := range msgs {
		mm := m.(map[string]any)
		got = append(got, fmt.Sprintf("%v|%v|%v", mm["role"], mm["content"], mm["tool_call_id"]))
	}
	want := []string{
		"system|You are Snow.|<nil>",
		"user|Read a.go|<nil>",
		"assistant|Reading.|<nil>",
		"tool|package a|call_1",
		"user|Now summarize.|<nil>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("messages\n got %q\nwant %q", got, want)
	}
	calls, _ := msgs[2].(map[string]any)["tool_calls"].([]any)
	if len(calls) != 1 {
		t.Fatalf("tool_calls %v", calls)
	}
	call := calls[0].(map[string]any)
	fn := call["function"].(map[string]any)
	if call["id"] != "call_1" || call["type"] != "function" || fn["name"] != "read_file" || fn["arguments"] != `{"path":"a.go"}` {
		t.Fatalf("tool call %v", call)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools %v", body["tools"])
	}
	def := tools[0].(map[string]any)["function"].(map[string]any)
	if tools[0].(map[string]any)["type"] != "function" || def["name"] != "read_file" || def["description"] != "Read a file." {
		t.Fatalf("tool %v", tools[0])
	}
}

// TestChatToolResultError: a failed tool result says so in its text, as the
// API has no error flag.
func TestChatToolResultError(t *testing.T) {
	cs := newChatServer(t, "chat_text.sse")
	req := provider.Request{Model: "m", Messages: []provider.Message{
		{Role: provider.RoleAssistant, Content: []provider.Block{&provider.ToolUse{ID: "c", Name: "run"}}},
		{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolResult{ToolUseID: "c", Content: "boom", IsError: true}}},
	}}
	if _, err := collectChat(t, chatProvider(cs.URL, false), req); err != nil {
		t.Fatal(err)
	}
	_, _, body := cs.last()
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages %v", msgs)
	}
	if got := msgs[1].(map[string]any)["content"]; got != "Error: boom" {
		t.Fatalf("tool message %v", got)
	}
	// A tool call with no input still sends valid JSON arguments.
	calls, _ := msgs[0].(map[string]any)["tool_calls"].([]any)
	if args := calls[0].(map[string]any)["function"].(map[string]any)["arguments"]; args != "{}" {
		t.Fatalf("arguments %v", args)
	}
}

// TestChatNoKey: a service that takes no key gets no Authorization header,
// and one that takes a key gets that key, under its own name.
func TestChatNoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-env-WRONG")
	for _, tt := range []struct {
		name  string
		noKey bool
		auth  string
	}{
		{"no key", true, ""},
		{"key", false, "Bearer " + compatKey},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cs := newChatServer(t, "chat_text.sse")
			if _, err := collectChat(t, chatProvider(cs.URL, tt.noKey), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}}); err != nil {
				t.Fatal(err)
			}
			if auth, _, _ := cs.last(); auth != tt.auth {
				t.Fatalf("Authorization %q, want %q", auth, tt.auth)
			}
		})
	}
}

// TestChatErrors: each failure maps to its kind, and no error holds the
// key, even when the service echoes it.
func TestChatErrors(t *testing.T) {
	echo := errBody("invalid_api_key", "invalid_request_error", "Bad key "+compatKey+" for "+compatKey)
	for _, tt := range []struct {
		name   string
		status int
		body   string
		header map[string]string
		want   provider.Kind
		retry  time.Duration
		text   string
	}{
		{"401", 401, errBody("invalid_api_key", "invalid_request_error", "Incorrect API key."), nil, provider.KindBadKey, 0, "Incorrect API key"},
		{"404", 404, errBody("model_not_found", "invalid_request_error", "model 'x' not found"), nil, provider.KindModelNotFound, 0, "not found"},
		{"429", 429, errBody("rate_limit_exceeded", "requests", "Slow down."), map[string]string{"retry-after": "30"}, provider.KindRateLimited, 30 * time.Second, "Slow down"},
		{"500", 500, errBody("server_error", "server_error", "The server had an error."), nil, provider.KindOther, 0, "server had an error"},
		{"echoed key", 401, echo, nil, provider.KindBadKey, 0, "Bad key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := rawServer(t, tt.status, "application/json", tt.body, tt.header)
			_, err := collectChat(t, chatProvider(strings.TrimSuffix(srv.URL, "/"), false), provider.Request{Model: "m"})
			var pe *provider.Error
			if !errors.As(err, &pe) || pe.Kind != tt.want || pe.RetryAfter != tt.retry {
				t.Fatalf("got %v (%#v), want kind %v retry %v", err, pe, tt.want, tt.retry)
			}
			if !strings.Contains(err.Error(), tt.text) {
				t.Errorf("message %q lacks %q", err.Error(), tt.text)
			}
			if strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), compatKey) {
				t.Fatalf("the error holds the key: %v", err)
			}
		})
	}
	t.Run("closed connection", func(t *testing.T) {
		_, err := collectChat(t, chatProvider(strings.TrimSuffix(closedURL(), "/"), false), provider.Request{Model: "m"})
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
			t.Fatalf("error = %v, want KindNetwork", err)
		}
	})
	t.Run("bare string error inside the stream", func(t *testing.T) {
		// Ollama and llama.cpp send {"error": "text"}, not an object.
		cs := newChatServer(t, "chat_error_string.sse")
		_, err := collectChat(t, chatProvider(cs.URL, false), provider.Request{Model: "m"})
		var pe *provider.Error
		if !errors.As(err, &pe) || !strings.Contains(err.Error(), "model 'x' not found") {
			t.Fatalf("error = %v (%#v)", err, pe)
		}
	})
	t.Run("bare string error echoing the key", func(t *testing.T) {
		srv := rawServer(t, 200, "text/event-stream", "data: {\"error\":\"bad key "+compatKey+"\"}\n\n", nil)
		_, err := collectChat(t, chatProvider(strings.TrimSuffix(srv.URL, "/"), false), provider.Request{Model: "m"})
		if err == nil || !strings.Contains(err.Error(), "bad key") || strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), compatKey) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("error inside the stream", func(t *testing.T) {
		body := "data: {\"error\":{\"message\":\"Overloaded " + compatKey + "\",\"type\":\"server_error\",\"code\":\"server_is_overloaded\"}}\n\n"
		srv := rawServer(t, 200, "text/event-stream", body, nil)
		_, err := collectChat(t, chatProvider(strings.TrimSuffix(srv.URL, "/"), false), provider.Request{Model: "m"})
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindOverloaded || strings.Contains(err.Error(), compatKey) {
			t.Fatalf("error = %v (%#v)", err, pe)
		}
	})
}

// TestChatRefusesRedirect: a chat service that redirects to another host
// does not get the key there.
func TestChatRefusesRedirect(t *testing.T) {
	var gotKey atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey.Store(r.Header.Get("Authorization") != "")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(other.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/v1/chat/completions", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	_, err := collectChat(t, chatProvider(srv.URL, false), provider.Request{Model: "m"})
	if err == nil || gotKey.Load() {
		t.Fatalf("redirect followed: err %v, key sent %v", err, gotKey.Load())
	}
}

// sseChunk is one chunk of a reply: choices is its JSON, and extra more
// top-level fields, such as usage.
func sseChunk(choices, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"m","choices":` + choices + extra + "}\n\n"
}

// inlineChat streams the reply in body, which a test writes in place.
func inlineChat(t *testing.T, body string) []provider.Event {
	t.Helper()
	srv := rawServer(t, 200, "text/event-stream", body, nil)
	evs, err := collectChat(t, chatProvider(strings.TrimSuffix(srv.URL, "/"), false), provider.Request{Model: "m", Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	return evs
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

// TestChatMessageShapes: text blocks join into one message, an empty text
// block sends nothing, and a block in the wrong role is an error, not a
// silent drop.
func TestChatMessageShapes(t *testing.T) {
	textBlocks := func(texts ...string) []provider.Block {
		var bs []provider.Block
		for _, s := range texts {
			bs = append(bs, &provider.Text{Text: s})
		}
		return bs
	}
	roles := func(msgs ...provider.Message) []string {
		params, err := toChatParams(provider.Request{Model: "m", Messages: msgs})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(params.Messages)
		if err != nil {
			t.Fatal(err)
		}
		var out []map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range out {
			got = append(got, fmt.Sprintf("%v|%v", m["role"], m["content"]))
		}
		return got
	}
	if got, want := roles(provider.Message{Role: provider.RoleUser, Content: textBlocks("one", "two")}), []string{"user|one\n\ntwo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("joined %q, want %q", got, want)
	}
	if got := roles(provider.Message{Role: provider.RoleUser, Content: textBlocks("", "")}); len(got) != 0 {
		t.Fatalf("an empty text sent %q", got)
	}
	for _, tt := range []struct {
		name string
		msg  provider.Message
	}{
		{"tool use from the user", provider.Message{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolUse{ID: "c", Name: "run"}}}},
		{"tool result from the assistant", provider.Message{Role: provider.RoleAssistant, Content: []provider.Block{&provider.ToolResult{ToolUseID: "c"}}}},
	} {
		if _, err := toChatParams(provider.Request{Model: "m", Messages: []provider.Message{tt.msg}}); err == nil {
			t.Errorf("%s: no error", tt.name)
		}
	}
}

// TestChatNoRetry: a failed request is sent once. Snow never retries by
// itself, so a 429 or a 500 reaches the user after one request.
func TestChatNoRetry(t *testing.T) {
	for _, status := range []int{429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var n atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, errBody("x", "x", "no"))
			}))
			t.Cleanup(srv.Close)
			if _, err := collectChat(t, chatProvider(srv.URL, false), provider.Request{Model: "m"}); err == nil {
				t.Fatal("no error")
			}
			if got := n.Load(); got != 1 {
				t.Fatalf("%d requests, want 1", got)
			}
		})
	}
}
