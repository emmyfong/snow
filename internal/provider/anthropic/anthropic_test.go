package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
)

const testKey = "sk-ant-api03-SECRET-test-key"

// keys is a provider.KeySource with one fixed key.
type keys string

func (k keys) APIKey(string) (string, error) { return string(k), nil }

// sseServer answers /v1/messages with a fixture and records the request.
func sseServer(t *testing.T, fixture string, got *map[string]any) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != testKey {
			http.Error(w, "wrong key", http.StatusUnauthorized)
			return
		}
		if got != nil {
			_ = json.NewDecoder(r.Body).Decode(got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTest(url string) *Provider { return New(keys(testKey), WithBaseURL(url)) }

// collect reads a stream to its end and returns the events and the error
// that ended it (nil for io.EOF).
func collect(t *testing.T, s provider.Stream) ([]provider.Event, error) {
	t.Helper()
	defer func() { _ = s.Close() }()
	var evs []provider.Event
	for {
		ev, err := s.Next()
		if errors.Is(err, io.EOF) {
			return evs, nil
		}
		if err != nil {
			return evs, err
		}
		evs = append(evs, ev)
	}
}

func streamFixture(t *testing.T, fixture string, req provider.Request) ([]provider.Event, map[string]any, error) {
	t.Helper()
	body := map[string]any{}
	srv := sseServer(t, fixture, &body)
	s, err := newTest(srv.URL).Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := collect(t, s)
	return evs, body, err
}

func only[T provider.Event](evs []provider.Event) []T {
	var out []T
	for _, e := range evs {
		if v, ok := e.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

func TestStreamText(t *testing.T) {
	evs, body, err := streamFixture(t, "text.sse", provider.Request{
		Model: "claude-opus-5-5", Effort: provider.EffortHigh, System: "Be brief.",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "Hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, d := range only[*provider.TextDelta](evs) {
		text += d.Text
	}
	if text != "Hello, world" {
		t.Fatalf("text = %q", text)
	}
	if _, ok := evs[0].(*provider.Start); !ok {
		t.Fatalf("first event %T, want *Start", evs[0])
	}
	if s := evs[len(evs)-1].(*provider.Stop); s.Reason != provider.StopEndTurn {
		t.Fatalf("stop = %v", s.Reason)
	}
	// The request carries what Snow asked for.
	if body["model"] != "claude-opus-5-5" || body["stream"] != true {
		t.Fatalf("request model/stream = %v/%v", body["model"], body["stream"])
	}
	if oc, _ := body["output_config"].(map[string]any); oc["effort"] != "high" {
		t.Fatalf("output_config = %v", body["output_config"])
	}
	if th, _ := body["thinking"].(map[string]any); th["type"] != "adaptive" || th["display"] != "summarized" {
		t.Fatalf("thinking = %v", body["thinking"])
	}
}

func TestStreamThinking(t *testing.T) {
	evs, _, err := streamFixture(t, "thinking.sse", provider.Request{Model: "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	var deltas string
	for _, d := range only[*provider.ThinkingDelta](evs) {
		deltas += d.Text
	}
	blocks := only[*provider.Thinking](evs)
	want := &provider.Thinking{Text: "The user greets me. I reply.", Signature: "sig-abc"}
	if deltas != want.Text || len(blocks) != 1 || !reflect.DeepEqual(blocks[0], want) {
		t.Fatalf("thinking deltas %q, blocks %v", deltas, blocks)
	}
}

func TestStreamToolUse(t *testing.T) {
	evs, _, err := streamFixture(t, "tool_use.sse", provider.Request{
		Model: "claude-opus-5-5",
		Tools: []provider.ToolDef{{Name: "read", Description: "Read a file", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := only[*provider.ToolUse](evs)
	if len(calls) != 1 || calls[0].ID != "toolu_01" || calls[0].Name != "read" {
		t.Fatalf("tool calls = %v", calls)
	}
	var input map[string]string
	if err := json.Unmarshal(calls[0].Input, &input); err != nil || input["path"] != "main.go" {
		t.Fatalf("input %s: %v", calls[0].Input, err)
	}
	if s := evs[len(evs)-1].(*provider.Stop); s.Reason != provider.StopToolUse {
		t.Fatalf("stop = %v", s.Reason)
	}
}

// TestToolSchemaSent: the tool's JSON Schema reaches the API whole.
func TestToolSchemaSent(t *testing.T) {
	_, body, _ := streamFixture(t, "text.sse", provider.Request{
		Model: "m",
		Tools: []provider.ToolDef{{Name: "read", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}},
	})
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", body["tools"])
	}
	schema := tools[0].(map[string]any)["input_schema"].(map[string]any)
	if schema["type"] != "object" || schema["additionalProperties"] != false || fmt.Sprint(schema["required"]) != "[path]" || schema["properties"] == nil {
		t.Fatalf("input_schema = %v", schema)
	}
}

// TestMessagesSent: every block kind goes back to the API in its own shape.
func TestMessagesSent(t *testing.T) {
	_, body, _ := streamFixture(t, "text.sse", provider.Request{
		Model: "m",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "read main.go"}}},
			{Role: provider.RoleAssistant, Content: []provider.Block{
				&provider.Thinking{Text: "need the file", Signature: "sig"},
				&provider.ToolUse{ID: "toolu_01", Name: "read", Input: json.RawMessage(`{"path":"main.go"}`)},
			}},
			{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolResult{ToolUseID: "toolu_01", Content: "package main", IsError: false}}},
		},
	})
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %v", body["messages"])
	}
	types := func(m any) (out []string) {
		for _, c := range m.(map[string]any)["content"].([]any) {
			out = append(out, c.(map[string]any)["type"].(string))
		}
		return out
	}
	if got := fmt.Sprint(types(msgs[0]), types(msgs[1]), types(msgs[2])); got != "[text] [thinking tool_use] [tool_result]" {
		t.Fatalf("block types = %s", got)
	}
}

func TestStreamUsage(t *testing.T) {
	evs, _, err := streamFixture(t, "usage.sse", provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	u := only[*provider.Usage](evs)
	want := provider.Usage{Input: 3, Output: 7, CacheRead: 2000, CacheWrite: 500}
	if len(u) == 0 || *u[len(u)-1] != want {
		t.Fatalf("last usage = %v, want %v", u, want)
	}
}

// TestResponseModel: the stream reports the model the API says it used,
// even when it differs from the one asked for.
func TestResponseModel(t *testing.T) {
	evs, _, err := streamFixture(t, "model.sse", provider.Request{Model: "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	if s := only[*provider.Start](evs); len(s) != 1 || s[0].Model != "claude-opus-5-5-20261001" {
		t.Fatalf("start = %v", s)
	}
}

// TestStreamCutOff: a stream that ends without message_stop is a network
// error, so the turn can show the reply as cut off.
func TestStreamCutOff(t *testing.T) {
	evs, _, err := streamFixture(t, "cut.sse", provider.Request{Model: "m"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
	if len(only[*provider.TextDelta](evs)) != 1 {
		t.Fatalf("the partial text was lost: %v", evs)
	}
}

// errServer answers every request with status, an API error body, and the
// given headers.
func errServer(t *testing.T, status int, errType, msg string, header map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"type":"error","error":{"type":%q,"message":%q}}`, errType, msg)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		status  int
		errType string
		msg     string
		header  map[string]string
		want    provider.Kind
		retry   time.Duration
	}{
		{401, "authentication_error", "invalid x-api-key", nil, provider.KindBadKey, 0},
		{403, "permission_error", "no access", nil, provider.KindBadKey, 0},
		{404, "not_found_error", "model: claude-old", nil, provider.KindModelNotFound, 0},
		{429, "rate_limit_error", "slow down", map[string]string{"retry-after": "30"}, provider.KindRateLimited, 30 * time.Second},
		{529, "overloaded_error", "Overloaded", nil, provider.KindOverloaded, 0},
		{400, "invalid_request_error", "prompt is too long: 1200000 tokens > 1000000 maximum", nil, provider.KindContextTooLong, 0},
		{413, "request_too_large", "request too large", nil, provider.KindContextTooLong, 0},
		{400, "invalid_request_error", "messages: field required", nil, provider.KindOther, 0},
		{500, "api_error", "internal", nil, provider.KindOther, 0},
	}
	for _, tt := range tests {
		srv := errServer(t, tt.status, tt.errType, tt.msg, tt.header)
		_, err := newTest(srv.URL).Stream(context.Background(), provider.Request{Model: "m"})
		if err == nil {
			s, _ := newTest(srv.URL).Stream(context.Background(), provider.Request{Model: "m"})
			_, err = collect(t, s)
		}
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != tt.want || pe.RetryAfter != tt.retry {
			t.Errorf("%d %s: got %v (%#v), want kind %v retry %v", tt.status, tt.errType, err, pe, tt.want, tt.retry)
			continue
		}
		if !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%d: message %q lacks the API's %q", tt.status, err.Error(), tt.msg)
		}
	}
}

// TestErrorMidStream: an error event inside a 200 stream maps by its type.
func TestErrorMidStream(t *testing.T) {
	evs, _, err := streamFixture(t, "error_midstream.sse", provider.Request{Model: "m"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindOverloaded {
		t.Fatalf("error = %v, want KindOverloaded", err)
	}
	if len(only[*provider.TextDelta](evs)) != 1 {
		t.Fatalf("events before the error were lost: %v", evs)
	}
}

func TestNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	s, err := newTest(url).Stream(context.Background(), provider.Request{Model: "m"})
	if err == nil {
		_, err = collect(t, s)
	}
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

// TestStreamCancel: canceling mid-stream ends it with the context error and
// leaves no goroutine behind.
func TestStreamCancel(t *testing.T) {
	sent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The server notices a client hang-up only once the body is read.
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := os.ReadFile(filepath.Join("testdata", "cut.sse"))
		_, _ = w.Write(data)
		w.(http.Flusher).Flush()
		close(sent)
		<-r.Context().Done()
	}))
	defer srv.Close()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	s, err := New(keys(testKey), WithBaseURL(srv.URL), WithHTTPClient(client)).Stream(ctx, provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	<-sent
	cancel()
	if _, err := s.Next(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Next after cancel = %v, want context.Canceled", err)
	}
	if err := s.Close(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Close = %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		buf := make([]byte, 1<<16)
		t.Fatalf("%d goroutines after cancel, %d before:\n%s", n, before, buf[:runtime.Stack(buf, true)])
	}
}

// modelServer lists two models, one retired, and checks the key.
func modelServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[
			{"type":"model","id":"claude-opus-5-5","display_name":"Claude Opus 5.5","created_at":"2026-08-01T00:00:00Z","lifecycle":"active","max_input_tokens":1000000,"max_tokens":128000},
			{"type":"model","id":"claude-2","display_name":"Claude 2","created_at":"2023-07-11T00:00:00Z","lifecycle":"retired","max_input_tokens":100000,"max_tokens":4096}
		],"has_more":false,"first_id":"claude-opus-5-5","last_id":"claude-2"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestModels(t *testing.T) {
	got, err := newTest(modelServer(t).URL).Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []provider.Model{{ID: "claude-opus-5-5", DisplayName: "Claude Opus 5.5", ContextWindow: 1000000}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Models = %v, want %v", got, want)
	}
}

func TestCheckKey(t *testing.T) {
	srv := modelServer(t)
	if err := newTest(srv.URL).CheckKey(context.Background()); err != nil {
		t.Fatalf("good key: %v", err)
	}
	err := New(keys("sk-ant-wrong"), WithBaseURL(srv.URL)).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey {
		t.Fatalf("bad key: %v, want KindBadKey", err)
	}
}

// noKeys fails as a key store with nothing saved does.
type noKeys struct{}

var errNoKey = errors.New("no API key saved")

func (noKeys) APIKey(string) (string, error) { return "", errNoKey }

// TestNoKey: with no key saved, the request is not sent and the store's
// error comes back.
func TestNoKey(t *testing.T) {
	if _, err := New(noKeys{}).Stream(context.Background(), provider.Request{Model: "m"}); !errors.Is(err, errNoKey) {
		t.Fatalf("Stream = %v, want the key store's error", err)
	}
}

// TestKeyNotLogged: no error text and no log line holds the key.
func TestKeyNotLogged(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	var errs []error
	for _, status := range []int{400, 401, 404, 429, 500, 529} {
		srv := errServer(t, status, "x_error", "refused", nil)
		_, err := newTest(srv.URL).Stream(context.Background(), provider.Request{Model: "m"})
		if err == nil {
			s, _ := newTest(srv.URL).Stream(context.Background(), provider.Request{Model: "m"})
			_, err = collect(t, s)
		}
		errs = append(errs, err, newTest(srv.URL).CheckKey(context.Background()))
		_, err = newTest(srv.URL).Models(context.Background())
		errs = append(errs, err)
	}
	_, _, err := streamFixture(t, "error_midstream.sse", provider.Request{Model: "m"})
	errs = append(errs, err)
	for _, err := range errs {
		if err == nil {
			t.Fatal("an error case returned no error")
		}
		for _, text := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
			if strings.Contains(text, testKey) {
				t.Fatalf("error holds the key: %s", text)
			}
		}
		var pe *provider.Error
		if errors.As(err, &pe) && pe.Err != nil && strings.Contains(fmt.Sprintf("%#v", pe.Err), testKey) {
			t.Fatalf("the cause holds the key")
		}
	}
	if strings.Contains(logs.String(), testKey) {
		t.Fatalf("a log line holds the key: %s", logs.String())
	}
}
