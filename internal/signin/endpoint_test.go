package signin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider/openai"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// TestEndpoints: an endpoint logs in, is listed, and registers under its
// own name; one that takes no key is connected with none saved.
func TestEndpoints(t *testing.T) {
	keyring.MockInit()
	t.Setenv("GW_API_KEY", "")
	srv := providertest.NewKeyServer(t, goodKey)
	store := credential.New(t.TempDir(), nil)
	ps := Endpoints([]openai.Compat{
		{Name: "gw", BaseURL: srv.URL},
		{Name: "local", BaseURL: srv.URL, NoKey: true},
	})
	if len(ps) != 2 || ps[0].Name != "gw" || ps[0].Title != "gw" || ps[0].NoKey || !ps[1].NoKey {
		t.Fatalf("providers %+v", ps)
	}
	st := Statuses(ps, store)
	if st[0].Connected() || !errors.Is(st[0].Err, credential.ErrNoKey) || !st[1].Connected() {
		t.Fatalf("statuses before login %+v", st)
	}
	if _, err := ps[0].Login(context.Background(), store, key(t, badKey)); err == nil {
		t.Fatal("a rejected key was saved")
	}
	if _, err := ps[0].Login(context.Background(), store, key(t, goodKey)); err != nil {
		t.Fatal(err)
	}
	if got, _, err := store.Get("gw"); err != nil || got.Reveal() != goodKey {
		t.Fatalf("the key is not saved under the endpoint's name: %v", err)
	}
	if _, err := ps[1].Login(context.Background(), store, key(t, goodKey)); !errors.Is(err, ErrNoKeyNeeded) {
		t.Fatalf("login to a provider with no key: %v", err)
	}
	reg, err := Registry(append(ProvidersAt(BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL}), ps...), store)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anthropic", "openai", "gw", "local"} {
		if p, ok := reg.Get(name); !ok || p.Name() != name {
			t.Fatalf("registry has no %s", name)
		}
	}
}

// TestEndpointUncheckedKey: a service with no model list cannot check a
// key; it is saved, and the result and the /login notice say so.
func TestEndpointUncheckedKey(t *testing.T) {
	keyring.MockInit()
	t.Setenv("GW_API_KEY", "")
	none := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(none.Close)
	ps := Endpoints([]openai.Compat{{Name: "gw", BaseURL: none.URL}, {Name: "local", BaseURL: none.URL, NoKey: true}})
	dir := t.TempDir()
	res, err := ps[0].Login(context.Background(), credential.New(dir, nil), key(t, goodKey))
	if err != nil || !res.Unchecked {
		t.Fatalf("Login = %+v, %v; want saved and unchecked", res, err)
	}

	a := NewAuth(ps, t.TempDir())
	lr, err := a.Login(context.Background(), "gw", goodKey)
	if err != nil || !strings.Contains(lr.Notice, UncheckedText) {
		t.Fatalf("Auth.Login = %+v, %v", lr, err)
	}
	if _, err := a.Login(context.Background(), "local", goodKey); !errors.Is(err, ErrNoKeyNeeded) {
		t.Fatalf("login to a provider with no key: %v", err)
	}
	list := a.Providers()
	if len(list) != 2 || list[0].NoKey || !list[1].NoKey || !list[1].Connected || list[1].Source != "" {
		t.Fatalf("providers %+v", list)
	}
}
