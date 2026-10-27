package openai

import (
	"context"
	"errors"
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

const compatKey = "sk-or-SECRET-compat-key"

// namedKeys holds keys by provider name, so a test sees which name a
// provider asks for.
type namedKeys map[string]string

func (k namedKeys) APIKey(name string) (string, error) {
	if v, ok := k[name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("no key for %s", name)
}

// compatServer is a compatible service. It lists ids, answers /responses
// with the tool call fixture, and records each request's path and
// Authorization header. It does not check the key.
type compatServer struct {
	*httptest.Server
	mu    sync.Mutex
	auths []string
	paths []string
}

func newCompatServer(t *testing.T, ids ...string) *compatServer {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tool_call.sse"))
	if err != nil {
		t.Fatal(err)
	}
	cs := &compatServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.auths = append(cs.auths, r.Header.Get("Authorization"))
		cs.paths = append(cs.paths, r.URL.Path)
		cs.mu.Unlock()
		switch r.URL.Path {
		case "/v1/models":
			entries := make([]string, len(ids))
			for i, id := range ids {
				entries[i] = fmt.Sprintf(`{"id":%q,"object":"model","created":1761200000,"owned_by":"gateway"}`, id)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"object":"list","data":[%s]}`, strings.Join(entries, ","))
		case "/v1/responses":
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *compatServer) seen() (auths, paths []string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return append([]string(nil), cs.auths...), append([]string(nil), cs.paths...)
}

func TestCompatStream(t *testing.T) {
	tests := []struct {
		name  string
		noKey bool
		auth  string
	}{
		{"no key", true, ""},
		{"key", false, "Bearer " + compatKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := newCompatServer(t)
			// Under another name, the key must not be sent.
			keys := namedKeys{"omniroute": compatKey, Name: "sk-proj-WRONG"}
			p := NewCompatible(Compat{Name: "omniroute", BaseURL: cs.URL + "/v1", NoKey: tt.noKey}, keys)
			if p.Name() != "omniroute" {
				t.Fatalf("Name() = %q", p.Name())
			}
			s, err := p.Stream(context.Background(), provider.Request{Model: "cc/claude-x", Messages: []provider.Message{user("hi")}})
			if err != nil {
				t.Fatal(err)
			}
			evs, err := providertest.Collect(t, s)
			if err != nil {
				t.Fatal(err)
			}
			if tools := providertest.Only[*provider.ToolUse](evs); len(tools) != 2 || tools[0].Name != "read" {
				t.Fatalf("tool calls %#v", tools)
			}
			auths, paths := cs.seen()
			if len(paths) != 1 || paths[0] != "/v1/responses" || auths[0] != tt.auth {
				t.Fatalf("requests %v with auth %q, want /v1/responses with %q", paths, auths, tt.auth)
			}
		})
	}
}

func TestCompatModels(t *testing.T) {
	cs := newCompatServer(t, "cc/claude-x", "gpt-oss-120b", "text-embedding-3-small")
	p := NewCompatible(Compat{Name: "omniroute", BaseURL: cs.URL + "/v1", NoKey: true}, namedKeys{})
	ms, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range ms {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, " ") != "cc/claude-x gpt-oss-120b text-embedding-3-small" {
		t.Fatalf("models %v; a compatible service's whole list is offered", ids)
	}

	listed := NewCompatible(Compat{Name: "omniroute", BaseURL: cs.URL + "/v1", Models: []string{"auto/best-free", "gpt-5.5"}}, namedKeys{})
	ms, err = listed.Models(context.Background())
	if err != nil || len(ms) != 2 || ms[0].ID != "auto/best-free" || ms[1].Efforts == nil {
		t.Fatalf("models from the config %+v, %v", ms, err)
	}
	if _, paths := cs.seen(); len(paths) != 1 {
		t.Fatalf("requests %v; the config's list needs none", paths)
	}
}

// TestCompatNoKey: an endpoint that takes a key, with none saved, fails
// with KindNoKey before any request.
func TestCompatNoKey(t *testing.T) {
	cs := newCompatServer(t)
	p := NewCompatible(Compat{Name: "omniroute", BaseURL: cs.URL + "/v1"}, namedKeys{})
	_, err := p.Stream(context.Background(), provider.Request{Model: "m"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNoKey {
		t.Fatalf("error %v, want KindNoKey", err)
	}
	if _, paths := cs.seen(); len(paths) != 0 {
		t.Fatalf("requests %v", paths)
	}
}

// TestCompatCheckKey: a rejected key is KindBadKey; a service with no
// model list passes, as nothing else checks a key for free.
func TestCompatCheckKey(t *testing.T) {
	bad := rawServer(t, http.StatusUnauthorized, "application/json", errBody("invalid_api_key", "auth", "bad key"), nil)
	err := NewCompatible(Compat{Name: "gw", BaseURL: bad.URL}, namedKeys{"gw": compatKey}).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey {
		t.Fatalf("bad key: %v", err)
	}
	none := rawServer(t, http.StatusNotFound, "text/plain", "404 page not found", nil)
	if err := NewCompatible(Compat{Name: "gw", BaseURL: none.URL}, namedKeys{"gw": compatKey}).CheckKey(context.Background()); err != nil {
		t.Fatalf("no model list: %v", err)
	}
	if err := New(namedKeys{Name: compatKey}, WithBaseURL(none.URL)).CheckKey(context.Background()); err == nil {
		t.Fatal("OpenAI itself with no model list passed the check")
	}
}

// TestCompatKeyNotLogged: no error from a compatible service holds its
// key, even when the service echoes the request back.
func TestCompatKeyNotLogged(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, errBody("bad", "invalid_request_error", "bad request with "+r.Header.Get("Authorization")))
	}))
	t.Cleanup(echo.Close)
	p := NewCompatible(Compat{Name: "gw", BaseURL: echo.URL}, namedKeys{"gw": compatKey})
	for _, err := range []error{streamErr(t, p), p.CheckKey(context.Background())} {
		if err == nil {
			t.Fatal("no error")
		}
		for e := err; e != nil; e = errors.Unwrap(e) {
			if strings.Contains(fmt.Sprintf("%v %+v %#v", e, e, e), compatKey) {
				t.Fatalf("the key is in the error: %v", e)
			}
		}
	}
}
