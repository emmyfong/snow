package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/openai/openai-go/v3"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providerhttp"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		body   string
		header map[string]string
		want   provider.Kind
		retry  time.Duration
		text   string // the error text holds it
	}{
		{401, errBody("invalid_api_key", "invalid_request_error", "Incorrect API key provided."), nil, provider.KindBadKey, 0, "Incorrect API key"},
		{403, errBody("", "invalid_request_error", "Project does not have access."), nil, provider.KindOther, 0, "does not have access"},
		{404, errBody("model_not_found", "invalid_request_error", "The model `gpt-old` does not exist."), nil, provider.KindModelNotFound, 0, "gpt-old"},
		{400, errBody("model_not_found", "invalid_request_error", "The model does not exist."), nil, provider.KindModelNotFound, 0, "does not exist"},
		{429, errBody("rate_limit_exceeded", "requests", "Rate limit reached."), map[string]string{"retry-after-ms": "1500"}, provider.KindRateLimited, 1500 * time.Millisecond, "Rate limit reached"},
		{429, errBody("rate_limit_exceeded", "tokens", "Slow down."), map[string]string{"retry-after": "30"}, provider.KindRateLimited, 30 * time.Second, "Slow down"},
		{429, errBody("insufficient_quota", "insufficient_quota", "You exceeded your current quota."), map[string]string{"retry-after": "30"}, provider.KindOther, 0, "current quota"},
		{400, errBody("context_length_exceeded", "invalid_request_error", "Input too long."), nil, provider.KindContextTooLong, 0, "Input too long"},
		{400, errBody("", "invalid_request_error", "This model's maximum context length is 400000 tokens."), nil, provider.KindContextTooLong, 0, "maximum context length"},
		{413, "", nil, provider.KindContextTooLong, 0, "HTTP 413"},
		{400, errBody("", "invalid_request_error", "Unknown parameter: 'foo'."), nil, provider.KindOther, 0, "Unknown parameter"},
		{500, errBody("server_error", "server_error", "The server had an error."), nil, provider.KindOther, 0, "server had an error"},
		{503, errBody("server_is_overloaded", "server_error", "Overloaded."), nil, provider.KindOverloaded, 0, "Overloaded"},
		// A proxy in front of the API answers with HTML and no error code.
		{502, "<html>Bad Gateway</html>", nil, provider.KindOverloaded, 0, "HTTP 502 Bad Gateway"},
		{503, "<html>busy</html>", nil, provider.KindOverloaded, 0, "HTTP 503 Service Unavailable"},
		{504, "", nil, provider.KindOverloaded, 0, "HTTP 504 Gateway Timeout"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d %s", tt.status, tt.text), func(t *testing.T) {
			srv := rawServer(t, tt.status, "application/json", tt.body, tt.header)
			err := streamErr(t, newTest(srv.URL))
			var pe *provider.Error
			if !errors.As(err, &pe) || pe.Kind != tt.want || pe.RetryAfter != tt.retry {
				t.Fatalf("got %v (%#v), want kind %v retry %v", err, pe, tt.want, tt.retry)
			}
			if !strings.Contains(err.Error(), tt.text) {
				t.Errorf("message %q lacks %q", err.Error(), tt.text)
			}
		})
	}
}

// TestLongMessageCut: a vendor's long message is cut to fit one line.
func TestLongMessageCut(t *testing.T) {
	srv := rawServer(t, 400, "application/json", errBody("", "invalid_request_error", strings.Repeat("x", 2000)), nil)
	err := streamErr(t, newTest(srv.URL))
	if err == nil || len(err.Error()) > providerhttp.MaxMessage+100 || !strings.HasSuffix(err.Error(), "...") {
		t.Fatalf("error text is %d bytes: %v", len(fmt.Sprint(err)), err)
	}
}

func TestNetworkError(t *testing.T) {
	err := streamErr(t, newTest(closedURL()))
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

// failingKeys is a provider.KeySource with nothing saved.
type failingKeys struct{}

func (failingKeys) APIKey(string) (string, error) {
	return "", errors.New("nothing saved for openai; set OPENAI_API_KEY")
}

// recorder is an http.RoundTripper that records requests and answers each
// with an empty model list, so no request leaves the process.
type recorder struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.reqs = append(r.reqs, req)
	r.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)),
		Request:    req,
	}, nil
}

// TestNoKey: without a key, every call fails with KindNoKey and sends
// nothing.
func TestNoKey(t *testing.T) {
	rec := &recorder{}
	p := New(failingKeys{}, WithHTTPClient(&http.Client{Transport: rec}))
	_, modelsErr := p.Models(context.Background())
	_, streamErr := p.Stream(context.Background(), provider.Request{Model: "gpt-6.1-sol"})
	for _, err := range []error{p.CheckKey(context.Background()), modelsErr, streamErr} {
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindNoKey || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
			t.Errorf("error = %v, want KindNoKey naming OPENAI_API_KEY", err)
		}
	}
	if len(rec.reqs) != 0 {
		t.Fatalf("%d requests went out without a key", len(rec.reqs))
	}
}

// TestIgnoresSDKEnvironment: the SDK's environment variables neither move
// the key to another host nor add headers, an organization, or a project.
func TestIgnoresSDKEnvironment(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the key went to OPENAI_BASE_URL")
	}))
	defer other.Close()
	t.Setenv("OPENAI_BASE_URL", other.URL)
	t.Setenv("OPENAI_API_KEY", "sk-from-env")
	t.Setenv("OPENAI_ORG_ID", "org-env")
	t.Setenv("OPENAI_PROJECT_ID", "proj-env")
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Leak: yes")

	rec := &recorder{}
	if err := New(providertest.Key(testKey), WithHTTPClient(&http.Client{Transport: rec})).CheckKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("%d requests, want 1", len(rec.reqs))
	}
	r := rec.reqs[0]
	if r.URL.Host != "api.openai.com" || r.Header.Get("Authorization") != "Bearer "+testKey {
		t.Fatalf("request to %s, key from Snow %v", r.URL.Host, r.Header.Get("Authorization") == "Bearer "+testKey)
	}
	for _, h := range []string{"OpenAI-Organization", "OpenAI-Project", "X-Leak"} {
		if v := r.Header.Get(h); v != "" {
			t.Errorf("header %s = %q came from the environment", h, v)
		}
	}
}

// TestRefusesCrossHostRedirect: a redirect to another host does not get
// the key.
func TestRefusesCrossHostRedirect(t *testing.T) {
	var leaked bool
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		leaked = leaked || r.Header.Get("Authorization") != ""
		mu.Unlock()
		_, _ = io.WriteString(w, `{"object":"list","data":[]}`)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	err := newTest(origin.URL).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if leaked {
		t.Fatal("the redirect target received the key")
	}
}

// TestKeyNotLogged: no error text, no error in the chain, and no log line
// holds the key or the request that carried it.
func TestKeyNotLogged(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	var errs []error
	for _, status := range []int{400, 401, 404, 429, 500, 502, 503} {
		srv := rawServer(t, status, "application/json", errBody("x_error", "x", "refused"), nil)
		p := newTest(srv.URL)
		_, err := p.Models(context.Background())
		errs = append(errs, streamErr(t, p), p.CheckKey(context.Background()), err)
	}
	for _, fixture := range []string{"failed.sse", "error_event.sse", "error_object.sse"} {
		_, _, err := streamFixture(t, fixture, provider.Request{Model: "gpt-6.1-sol"})
		errs = append(errs, err)
	}
	errs = append(errs, streamErr(t, newTest(closedURL())))
	for _, err := range errs {
		if err == nil {
			t.Fatal("an error case returned no error")
		}
		for _, text := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
			if strings.Contains(text, testKey) {
				t.Fatalf("error holds the key: %s", text)
			}
		}
		// The SDK's error holds the HTTP request and its key header, so it
		// may not be reachable from the error at all.
		if sdkErr := new(*sdk.Error); errors.As(err, sdkErr) {
			t.Fatalf("the SDK's error is in the chain of %v", err)
		}
	}
	if strings.Contains(logs.String(), testKey) {
		t.Fatalf("a log line holds the key: %s", logs.String())
	}
}
