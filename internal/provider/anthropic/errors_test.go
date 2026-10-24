package anthropic

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

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providerhttp"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestErrorMapping(t *testing.T) {
	json := func(errType, msg string) string {
		return fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q}}`, errType, msg)
	}
	tests := []struct {
		status int
		body   string
		header map[string]string
		want   provider.Kind
		retry  time.Duration
		text   string // the error text holds it
	}{
		{401, json("authentication_error", "invalid x-api-key"), nil, provider.KindBadKey, 0, "invalid x-api-key"},
		{403, json("permission_error", "no access"), nil, provider.KindBadKey, 0, "no access"},
		{404, json("not_found_error", "model: claude-old"), nil, provider.KindModelNotFound, 0, "model: claude-old"},
		{429, json("rate_limit_error", "slow down"), map[string]string{"retry-after": "30"}, provider.KindRateLimited, 30 * time.Second, "slow down"},
		{529, json("overloaded_error", "Overloaded"), nil, provider.KindOverloaded, 0, "Overloaded"},
		{400, json("invalid_request_error", "prompt is too long: 1200000 tokens > 1000000 maximum"), nil, provider.KindContextTooLong, 0, "prompt is too long"},
		{400, json("invalid_request_error", "input length and `max_tokens` exceed context limit: 990000 + 64000 > 1000000"), nil, provider.KindContextTooLong, 0, "exceed context limit"},
		{413, json("request_too_large", "request too large"), nil, provider.KindContextTooLong, 0, "request too large"},
		{400, json("invalid_request_error", "messages: field required"), nil, provider.KindOther, 0, "field required"},
		{500, json("api_error", "internal"), nil, provider.KindOther, 0, "internal"},
		// A proxy in front of the API answers with HTML and no error type.
		{502, "<html>Bad Gateway</html>", nil, provider.KindOverloaded, 0, "HTTP 502 Bad Gateway"},
		{503, "<html>busy</html>", nil, provider.KindOverloaded, 0, "HTTP 503 Service Unavailable"},
		{504, "", nil, provider.KindOverloaded, 0, "HTTP 504 Gateway Timeout"},
	}
	for _, tt := range tests {
		srv := rawServer(t, tt.status, "application/json", tt.body, tt.header)
		err := streamErr(t, newTest(srv.URL))
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != tt.want || pe.RetryAfter != tt.retry {
			t.Errorf("%d %s: got %v (%#v), want kind %v retry %v", tt.status, tt.body, err, pe, tt.want, tt.retry)
			continue
		}
		if !strings.Contains(err.Error(), tt.text) {
			t.Errorf("%d: message %q lacks %q", tt.status, err.Error(), tt.text)
		}
	}
}

// TestLongMessageCut: a vendor's long message is cut to fit one line.
func TestLongMessageCut(t *testing.T) {
	srv := errServer(t, 400, "invalid_request_error", strings.Repeat("x", 2000), nil)
	err := streamErr(t, newTest(srv.URL))
	if err == nil || len(err.Error()) > providerhttp.MaxMessage+100 || !strings.HasSuffix(err.Error(), "...") {
		t.Fatalf("error text is %d bytes: %v", len(fmt.Sprint(err)), err)
	}
}

// TestErrorMidStream: an error event inside a 200 stream maps by its type.
func TestErrorMidStream(t *testing.T) {
	evs, _, err := streamFixture(t, "error_midstream.sse", provider.Request{Model: "m"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindOverloaded {
		t.Fatalf("error = %v, want KindOverloaded", err)
	}
	if len(providertest.Only[*provider.TextDelta](evs)) != 1 {
		t.Fatalf("events before the error were lost: %v", evs)
	}
}

// closedURL is the address of a server that no longer listens.
func closedURL() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

func TestNetworkError(t *testing.T) {
	err := streamErr(t, newTest(closedURL()))
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

// recorder is an http.RoundTripper that records requests and answers each
// with a model list, so no request leaves the process.
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
		Body:       io.NopCloser(strings.NewReader(`{"data":[],"has_more":false}`)),
		Request:    req,
	}, nil
}

// TestIgnoresSDKEnvironment: the SDK's environment variables neither move
// the key to another host nor add a second credential.
func TestIgnoresSDKEnvironment(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the key went to ANTHROPIC_BASE_URL")
	}))
	defer other.Close()
	t.Setenv("ANTHROPIC_BASE_URL", other.URL)
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "env-token")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-from-env")

	rec := &recorder{}
	if err := New(providertest.Key(testKey), WithHTTPClient(&http.Client{Transport: rec})).CheckKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("%d requests, want 1", len(rec.reqs))
	}
	r := rec.reqs[0]
	if r.URL.Host != "api.anthropic.com" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != testKey {
		t.Fatalf("request to %s, Authorization %q, key from Snow %v", r.URL.Host, r.Header.Get("Authorization"), r.Header.Get("X-Api-Key") == testKey)
	}
}

// TestRefusesCrossHostRedirect: a redirect to another host does not get
// the key.
func TestRefusesCrossHostRedirect(t *testing.T) {
	var leaked bool
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		leaked = leaked || r.Header.Get("X-Api-Key") != ""
		mu.Unlock()
		_, _ = io.WriteString(w, `{"data":[],"has_more":false}`)
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
	for _, status := range []int{400, 401, 404, 429, 500, 502, 529} {
		srv := errServer(t, status, "x_error", "refused", nil)
		p := newTest(srv.URL)
		_, err := p.Models(context.Background())
		errs = append(errs, streamErr(t, p), p.CheckKey(context.Background()), err)
	}
	_, _, err := streamFixture(t, "error_midstream.sse", provider.Request{Model: "m"})
	errs = append(errs, err, streamErr(t, newTest(closedURL())))
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
