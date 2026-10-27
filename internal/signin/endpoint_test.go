package signin

import (
	"context"
	"errors"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// TestEndpoints: an endpoint logs in, is listed, and registers under its
// own name; one that takes no key is connected with none saved.
func TestEndpoints(t *testing.T) {
	keyring.MockInit()
	t.Setenv("GW_API_KEY", "")
	srv := providertest.NewKeyServer(t, goodKey)
	store := credential.New(t.TempDir(), nil)
	ps := Endpoints([]Endpoint{
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
