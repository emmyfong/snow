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
)

func TestAuthProviders(t *testing.T) {
	ps, store, dir := testSetup(t)
	auth := NewAuth(ps, dir)

	list := auth.Providers()
	if len(list) != len(ps) {
		t.Fatalf("Providers() returned %d providers, want %d", len(list), len(ps))
	}
	if list[0].Name != "anthropic" || list[0].Connected || list[0].Source != "" {
		t.Fatalf("empty store: %+v, want anthropic with no key", list[0])
	}
	if list[0].Err != "" {
		t.Errorf("no key shows Err %q, want an empty message", list[0].Err)
	}

	if _, err := store.Set("anthropic", key(t, goodKey)); err != nil {
		t.Fatal(err)
	}
	list = auth.Providers()
	if !list[0].Connected || list[0].Source != "OS credential store" {
		t.Fatalf("saved key: %+v, want connected in the OS store", list[0])
	}
	if list[0].EnvVar != "" {
		t.Errorf("saved key shows EnvVar %q, want empty", list[0].EnvVar)
	}

	t.Setenv("OPENAI_API_KEY", goodKey)
	list = auth.Providers()
	if !list[1].Connected || list[1].Source != "environment" || list[1].EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("environment key: %+v, want connected from OPENAI_API_KEY", list[1])
	}
}

func TestAuthLogin(t *testing.T) {
	ps, store, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	ctx := context.Background()

	res, err := auth.Login(ctx, "anthropic", goodKey)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.Source != "OS credential store" || res.Shadowed {
		t.Errorf("Login result = %+v, want the OS store and no shadow", res)
	}
	k, src, err := store.Get("anthropic")
	if err != nil || k.Reveal() != goodKey || src != credential.SourceOS {
		t.Fatalf("saved key = %q from %v, error %v; want %q in the OS store", k, src, err, goodKey)
	}
}

// TestAuthLoginRejectsBadKey: a rejected key is not saved.
func TestAuthLoginRejectsBadKey(t *testing.T) {
	ps, store, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	_, err := auth.Login(context.Background(), "anthropic", badKey)
	if err == nil || !strings.Contains(err.Error(), "did not accept the key") {
		t.Fatalf("Login with a bad key: %v, want a rejection", err)
	}
	if _, _, err := store.Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after a rejected key: %v, want no key", err)
	}
}

// TestAuthLoginNetworkError: a check that cannot run saves nothing.
func TestAuthLoginNetworkError(t *testing.T) {
	keyring.MockInit()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close()
	dir := t.TempDir()
	auth := NewAuth(ProvidersAt(BaseURLs{Anthropic: dead.URL, OpenAI: dead.URL}), dir)
	_, err := auth.Login(context.Background(), "anthropic", goodKey)
	if err == nil || !strings.Contains(err.Error(), "could not check the key") {
		t.Fatalf("Login with no network: %v, want a check failure", err)
	}
	if _, _, err := credential.New(dir, nil).Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after a failed check: %v, want no key", err)
	}
}

// TestAuthLoginUnknownProvider: a name Snow does not know names the choices.
func TestAuthLoginUnknownProvider(t *testing.T) {
	ps, _, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	if _, err := auth.Login(context.Background(), "nope", goodKey); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Login with an unknown provider: %v, want ErrUnknownProvider", err)
	}
	if err := auth.Logout("nope"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Logout with an unknown provider: %v, want ErrUnknownProvider", err)
	}
}

// TestAuthLoginBadKeyShape: a key with white space is refused before a check.
func TestAuthLoginBadKeyShape(t *testing.T) {
	ps, _, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	if _, err := auth.Login(context.Background(), "anthropic", "sk two words"); !errors.Is(err, credential.ErrBadKey) {
		t.Fatalf("Login with a spaced key: %v, want ErrBadKey", err)
	}
}

// TestAuthLoginNotice: the store's pending one-time message goes to the next
// login result, once.
func TestAuthLoginNotice(t *testing.T) {
	ps, _, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	auth.Notice("API keys are saved in plain text.")
	ctx := context.Background()
	res, err := auth.Login(ctx, "anthropic", goodKey)
	if err != nil {
		t.Fatal(err)
	}
	if res.Notice != "API keys are saved in plain text." {
		t.Fatalf("Login notice = %q, want the store's message", res.Notice)
	}
	res, err = auth.Login(ctx, "anthropic", goodKey)
	if err != nil {
		t.Fatal(err)
	}
	if res.Notice != "" {
		t.Fatalf("second Login notice = %q, want it cleared", res.Notice)
	}
}

// TestAuthLogout: a saved key is removed; an environment key cannot be.
func TestAuthLogout(t *testing.T) {
	ps, store, dir := testSetup(t)
	auth := NewAuth(ps, dir)
	if _, err := store.Set("anthropic", key(t, goodKey)); err != nil {
		t.Fatal(err)
	}
	if err := auth.Logout("anthropic"); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, _, err := store.Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after Logout: %v, want no key", err)
	}
	if err := auth.Logout("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("second Logout: %v, want ErrNoKey", err)
	}

	t.Setenv("ANTHROPIC_API_KEY", goodKey)
	if err := auth.Logout("anthropic"); !errors.Is(err, credential.ErrInEnvironment) {
		t.Fatalf("Logout with an environment key: %v, want ErrInEnvironment", err)
	}
}
