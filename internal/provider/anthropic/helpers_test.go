package anthropic

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

const testKey = "sk-ant-api03-SECRET-test-key"

// sseServer answers /v1/messages with a fixture and records the request.
// Other paths, such as the model list, are not found, so the provider
// treats every model as unknown.
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
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
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

func newTest(url string) *Provider { return New(providertest.Key(testKey), WithBaseURL(url)) }

func streamFixture(t *testing.T, fixture string, req provider.Request) ([]provider.Event, map[string]any, error) {
	t.Helper()
	body := map[string]any{}
	srv := sseServer(t, fixture, &body)
	s, err := newTest(srv.URL).Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := providertest.Collect(t, s)
	return evs, body, err
}

// errServer answers every request with status, an API error body, and the
// given headers.
func errServer(t *testing.T, status int, errType, msg string, header map[string]string) *httptest.Server {
	t.Helper()
	body := fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q}}`, errType, msg)
	return rawServer(t, status, "application/json", body, header)
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

// streamErr sends one request and returns the error that ends it, from
// Stream or from reading the stream.
func streamErr(t *testing.T, p *Provider) error {
	t.Helper()
	s, err := p.Stream(context.Background(), provider.Request{Model: "m"})
	if err != nil {
		return err
	}
	_, err = providertest.Collect(t, s)
	return err
}

// model is one entry of a model list. efforts lists the effort levels it
// takes; nil leaves out the capabilities object, as an older API would.
type model struct {
	id, lifecycle string
	adaptive      bool
	efforts       []string
	noCaps        bool
}

func (m model) json() string {
	caps := ""
	if !m.noCaps {
		levels := map[string]bool{}
		for _, e := range m.efforts {
			levels[e] = true
		}
		effort := fmt.Sprintf(`"supported":%t`, len(m.efforts) > 0)
		for _, l := range []string{"low", "medium", "high", "xhigh", "max"} {
			effort += fmt.Sprintf(`,%q:{"supported":%t}`, l, levels[l])
		}
		caps = fmt.Sprintf(`,"capabilities":{"thinking":{"supported":%t,"types":{"adaptive":{"supported":%t},"enabled":{"supported":%t}}},"effort":{%s}}`,
			true, m.adaptive, !m.adaptive, effort)
	}
	return fmt.Sprintf(`{"type":"model","id":%q,"display_name":%q,"created_at":"2026-08-01T00:00:00Z","lifecycle":%q,"max_input_tokens":1000000,"max_tokens":128000%s}`,
		m.id, m.id, m.lifecycle, caps)
}

// testModels is the model list modelServer gives: a current model, an
// older one by snapshot ID that takes neither adaptive thinking nor
// effort, and a retired one.
var testModels = []model{
	{id: "claude-opus-5-5", lifecycle: "active", adaptive: true, efforts: []string{"low", "medium", "high", "max"}},
	{id: "claude-haiku-4-5-20251001", lifecycle: "active"},
	{id: "claude-2", lifecycle: "retired"},
}

// modelServer lists models, checks the key, and answers /v1/messages with
// a fixture. It records each request body and counts the model lists.
type modelServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	lists  int
}

func newModelServer(t *testing.T, models []model, fixture string) *modelServer {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	ms := &modelServer{}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			ms.mu.Lock()
			ms.lists++
			ms.mu.Unlock()
			entries := make([]string, len(models))
			for i, m := range models {
				entries[i] = m.json()
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":[%s],"has_more":false}`, strings.Join(entries, ","))
		case "/v1/messages":
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ms.mu.Lock()
			ms.bodies = append(ms.bodies, body)
			ms.mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ms.Close)
	return ms
}

// counts returns how many model lists and message requests arrived.
func (ms *modelServer) counts() (lists, messages int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.lists, len(ms.bodies)
}

// body returns the i-th message request body.
func (ms *modelServer) body(i int) map[string]any {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.bodies[i]
}
