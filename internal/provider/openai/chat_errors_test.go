package openai

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
)

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

// TestChatNoRetry: a failed request is sent once. Snow never retries by
// itself, so a 429 or a 500 reaches the user after one request.
func TestChatNoRetry(t *testing.T) {
	for _, status := range []int{429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var n atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
