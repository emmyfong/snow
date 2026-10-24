package signin

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
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
)

const (
	goodKey = "sk-good-SECRET"
	badKey  = "sk-bad-SECRET"
)

// vendor is a fake API for both providers: a model list that accepts only
// goodKey, by either vendor's key header.
func vendor(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Api-Key") != goodKey && r.Header.Get("Authorization") != "Bearer "+goodKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid key"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[],"has_more":false}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testSetup returns the built-in providers pointed at a fake API, and a
// credential store in a temporary folder with an in-memory OS store and
// no keys in the environment.
func testSetup(t *testing.T) ([]Provider, *credential.Store, string) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	srv := vendor(t)
	dir := t.TempDir()
	return ProvidersAt(BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL}), credential.New(dir, nil), dir
}

func key(t *testing.T, s string) credential.Key {
	t.Helper()
	k, err := credential.NewKey(s)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestLogin(t *testing.T) {
	ps, store, _ := testSetup(t)
	for _, p := range ps {
		src, err := p.Login(context.Background(), store, key(t, goodKey))
		if err != nil || src != credential.SourceOS {
			t.Fatalf("%s: Login = %v, %v", p.Name, src, err)
		}
		got, _, err := store.Get(p.Name)
		if err != nil || got.Reveal() != goodKey {
			t.Fatalf("%s: saved key not found: %v", p.Name, err)
		}
	}
}

// TestLoginRejected: a key the provider rejects is not saved, and the
// error says so without the key.
func TestLoginRejected(t *testing.T) {
	ps, store, dir := testSetup(t)
	for _, p := range ps {
		_, err := p.Login(context.Background(), store, key(t, badKey))
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey || !strings.Contains(err.Error(), "did not accept") {
			t.Fatalf("%s: Login = %v, want KindBadKey", p.Name, err)
		}
		if strings.Contains(fmt.Sprintf("%v %+v", err, err), badKey) {
			t.Fatalf("%s: error holds the key: %v", p.Name, err)
		}
		if _, _, err := store.Get(p.Name); !errors.Is(err, credential.ErrNoKey) {
			t.Fatalf("%s: a rejected key was saved: %v", p.Name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a credentials file was written: %v", err)
	}
}

func TestLogout(t *testing.T) {
	ps, store, _ := testSetup(t)
	p := ps[0]
	if _, err := p.Login(context.Background(), store, key(t, goodKey)); err != nil {
		t.Fatal(err)
	}
	if err := p.Logout(store); err != nil {
		t.Fatal(err)
	}
	if err := p.Logout(store); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("second Logout = %v, want ErrNoKey", err)
	}
	t.Setenv("ANTHROPIC_API_KEY", goodKey)
	if err := p.Logout(store); !errors.Is(err, credential.ErrInEnvironment) || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("Logout with the environment = %v", err)
	}
}

func TestStatuses(t *testing.T) {
	ps, store, _ := testSetup(t)
	if _, err := ps[0].Login(context.Background(), store, key(t, goodKey)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", goodKey)
	got := Statuses(ps, store)
	if len(got) != 2 || !got[0].Connected() || got[0].Source != credential.SourceOS ||
		!got[1].Connected() || got[1].Source != credential.SourceEnv {
		t.Fatalf("statuses = %+v", got)
	}
	t.Setenv("OPENAI_API_KEY", "")
	got = Statuses(ps, store)
	if got[1].Connected() || !errors.Is(got[1].Err, credential.ErrNoKey) {
		t.Fatalf("openai status = %+v, want ErrNoKey", got[1])
	}
}

func TestFind(t *testing.T) {
	for _, name := range []string{"anthropic", "openai"} {
		if p, err := Find(Providers(), name); err != nil || p.Name != name || p.KeyURL == "" || p.Title == "" {
			t.Errorf("Find(%s) = %+v, %v", name, p, err)
		}
	}
	if _, err := Find(Providers(), "gemini"); !errors.Is(err, ErrUnknownProvider) || !strings.Contains(err.Error(), "anthropic, openai") {
		t.Errorf("Find(gemini) = %v", err)
	}
}
