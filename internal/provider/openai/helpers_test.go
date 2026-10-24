package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const testKey = "sk-proj-SECRET-test-key"

// apiServer is a fake OpenAI API. It checks the key, lists ids as models,
// and answers /responses with a fixture. It records each response request.
type apiServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
}

func newAPIServer(t *testing.T, fixture string, ids ...string) *apiServer {
	t.Helper()
	var data []byte
	if fixture != "" {
		var err error
		if data, err = os.ReadFile(filepath.Join("testdata", fixture)); err != nil {
			t.Fatal(err)
		}
	}
	as := &apiServer{}
	as.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"Incorrect API key provided.","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`)
			return
		}
		switch r.URL.Path {
		case "/models":
			entries := make([]string, len(ids))
			for i, id := range ids {
				entries[i] = fmt.Sprintf(`{"id":%q,"object":"model","created":1761200000,"owned_by":"openai"}`, id)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"object":"list","data":[%s]}`, strings.Join(entries, ","))
		case "/responses":
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			as.mu.Lock()
			as.bodies = append(as.bodies, body)
			as.mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(as.Close)
	return as
}

// body returns the i-th response request body.
func (as *apiServer) body(i int) map[string]any {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.bodies[i]
}

func newTest(url string) *Provider { return New(providertest.Key(testKey), WithBaseURL(url)) }

// streamFixture streams req from a server that answers with fixture, and
// returns the events, the request body, and the error that ended the
// stream.
func streamFixture(t *testing.T, fixture string, req provider.Request) ([]provider.Event, map[string]any, error) {
	t.Helper()
	as := newAPIServer(t, fixture)
	s, err := newTest(as.URL).Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := providertest.Collect(t, s)
	return evs, as.body(0), err
}

// rawServer answers every request with status, body, and the headers.
func rawServer(t *testing.T, status int, contentType, body string, header map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// errBody is an API error body.
func errBody(code, errType, msg string) string {
	return fmt.Sprintf(`{"error":{"message":%q,"type":%q,"param":null,"code":%q}}`, msg, errType, code)
}

// streamErr sends one request and returns the error that ends it, from
// Stream or from reading the stream.
func streamErr(t *testing.T, p *Provider) error {
	t.Helper()
	s, err := p.Stream(context.Background(), provider.Request{Model: "gpt-6.1-sol"})
	if err != nil {
		return err
	}
	_, err = providertest.Collect(t, s)
	return err
}

// closedURL is the address of a server that no longer listens.
func closedURL() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// user is a user message with one text block.
func user(text string) provider.Message {
	return provider.Message{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: text}}}
}

// items returns the input items of a request body.
func items(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["input"].([]any)
	if !ok {
		t.Fatalf("input is %T, want a list", body["input"])
	}
	out := make([]map[string]any, len(raw))
	for i, r := range raw {
		out[i], _ = r.(map[string]any)
	}
	return out
}
