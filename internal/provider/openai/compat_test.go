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
	mu     sync.Mutex
	auths  []string
	paths  []string
	model  string      // of the last /responses request
	header http.Header // of the last request
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
		cs.header = r.Header.Clone()
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
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			cs.mu.Lock()
			cs.model = body.Model
			cs.mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *compatServer) lastModel() string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.model
}

func (cs *compatServer) lastHeader(name string) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.header.Get(name)
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
	// The SDK's environment defaults must not reach a compatible service.
	t.Setenv("OPENAI_API_KEY", "sk-env-WRONG")
	t.Setenv("OPENAI_BASE_URL", "http://env.invalid/v1")
	t.Setenv("OPENAI_ORG_ID", "org-env")
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
			if m := cs.lastModel(); m != "cc/claude-x" {
				t.Fatalf("model %q sent, want the ID with its slash", m)
			}
			if org := cs.lastHeader("Openai-Organization"); org != "" {
				t.Fatalf("OPENAI_ORG_ID reached the service: %q", org)
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

// TestCompatCheckKey: a rejected key fails, a 403 included, as gateways
// send it for a bad key; a service with no model list gives
// ErrKeyNotChecked. OpenAI itself keeps its own rules.
func TestCompatCheckKey(t *testing.T) {
	bad := rawServer(t, http.StatusUnauthorized, "application/json", errBody("invalid_api_key", "auth", "bad key"), nil)
	err := NewCompatible(Compat{Name: "gw", BaseURL: bad.URL}, namedKeys{"gw": compatKey}).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey {
		t.Fatalf("bad key: %v", err)
	}
	forbidden := rawServer(t, http.StatusForbidden, "application/json", errBody("forbidden", "auth", "no"), nil)
	if err := NewCompatible(Compat{Name: "gw", BaseURL: forbidden.URL}, namedKeys{"gw": compatKey}).CheckKey(context.Background()); err == nil {
		t.Fatal("a compatible service's 403 passed the check")
	}
	if err := New(namedKeys{Name: compatKey}, WithBaseURL(forbidden.URL)).CheckKey(context.Background()); err != nil {
		t.Fatalf("OpenAI's 403 is a restricted key, which works: %v", err)
	}
	none := rawServer(t, http.StatusNotFound, "text/plain", "404 page not found", nil)
	if err := NewCompatible(Compat{Name: "gw", BaseURL: none.URL}, namedKeys{"gw": compatKey}).CheckKey(context.Background()); !errors.Is(err, ErrKeyNotChecked) {
		t.Fatalf("no model list: %v, want ErrKeyNotChecked", err)
	}
	if err := New(namedKeys{Name: compatKey}, WithBaseURL(none.URL)).CheckKey(context.Background()); err == nil || errors.Is(err, ErrKeyNotChecked) {
		t.Fatalf("OpenAI itself with no model list: %v, want a failure", err)
	}
}

// TestCompatKeyNotLogged: no error from a compatible service holds its
// key, whichever field or path the service echoes it in.
func TestCompatKeyNotLogged(t *testing.T) {
	sse := func(events string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, events)
		}
	}
	created := "event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"r\",\"object\":\"response\",\"created_at\":1,\"status\":\"in_progress\",\"model\":\"m\",\"output\":[],\"usage\":null}}\n\n"
	tests := []struct {
		name    string
		handler http.HandlerFunc
		check   bool // also run CheckKey
	}{
		{"HTTP error message", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, errBody("bad", "invalid_request_error", "bad request with "+r.Header.Get("Authorization")))
		}, true},
		{"HTTP error code and type", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			auth := r.Header.Get("Authorization")
			_, _ = io.WriteString(w, errBody(auth, auth, ""))
		}, true},
		{"response.failed event", sse(created + "event: response.failed\ndata: {\"type\":\"response.failed\",\"sequence_number\":1,\"response\":{\"id\":\"r\",\"object\":\"response\",\"created_at\":1,\"status\":\"failed\",\"model\":\"m\",\"output\":[],\"usage\":null,\"error\":{\"code\":\"server_error\",\"message\":\"echo " + compatKey + "\"}}}\n\n"), false},
		{"error event", sse(created + "event: error\ndata: {\"type\":\"error\",\"code\":\"" + compatKey + "\",\"message\":\"echo " + compatKey + "\",\"param\":null,\"sequence_number\":1}\n\n"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			t.Cleanup(srv.Close)
			p := NewCompatible(Compat{Name: "gw", BaseURL: srv.URL}, namedKeys{"gw": compatKey})
			errs := []error{streamErr(t, p)}
			if tt.check {
				errs = append(errs, p.CheckKey(context.Background()))
			}
			for _, err := range errs {
				assertNoKey(t, err)
			}
		})
	}

	t.Run("network error", func(t *testing.T) {
		hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("proxy refused " + compatKey)
		})}
		p := NewCompatible(Compat{Name: "gw", BaseURL: "http://gw.invalid/v1"}, namedKeys{"gw": compatKey}, WithHTTPClient(hc))
		err := streamErr(t, p)
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
			t.Fatalf("error %v, want KindNetwork", err)
		}
		assertNoKey(t, err)
	})
}

// assertNoKey fails unless err is an error whose chain shows the key's
// mark and never the key.
func assertNoKey(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(err.Error(), keyMark) {
		t.Fatalf("error %q does not mark the blanked key", err)
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(fmt.Sprintf("%v %+v %#v", e, e, e), compatKey) {
			t.Fatalf("the key is in the error: %v", e)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
