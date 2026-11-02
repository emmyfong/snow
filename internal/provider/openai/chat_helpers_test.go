package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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

// dump writes events for a failure message.
func dump(evs []provider.Event) string {
	var parts []string
	for _, e := range evs {
		parts = append(parts, fmt.Sprintf("%T%+v", e, e))
	}
	return strings.Join(parts, " ")
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
