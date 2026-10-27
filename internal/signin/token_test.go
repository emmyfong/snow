package signin

import (
	"context"
	"testing"

	"github.com/emmyfong/snow/internal/credential"
)

// TestTokenLogin: a token is saved unchecked under its name, has a
// status, and gives the registry no provider.
func TestTokenLogin(t *testing.T) {
	ps, store, _ := testSetup(t)
	t.Setenv("OMNIROUTE_API_KEY", "")
	tokens := Tokens([]string{"omniroute"})
	res, err := tokens[0].Login(context.Background(), store, key(t, goodKey))
	if err != nil || !res.Unchecked || res.Source != credential.SourceOS {
		t.Fatalf("Login = %+v, %v; want saved unchecked", res, err)
	}
	if got, _, err := store.Get("omniroute"); err != nil || got.Reveal() != goodKey {
		t.Fatalf("saved token not found: %v", err)
	}
	if s := Statuses(tokens, store); !s[0].Connected() {
		t.Fatalf("status %+v, want connected", s[0])
	}
	reg, err := Registry(append(ps, tokens...), store)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("omniroute"); ok {
		t.Fatal("a token is in the provider registry")
	}
}
