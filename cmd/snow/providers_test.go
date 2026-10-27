package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/signin"
)

// writeUserConfig writes body to the config file in the test's home,
// whose .snow folder is dir.
func writeUserConfig(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestProvidersFromConfig: each [provider.<name>] table joins the built-in
// providers, in name order; one named like a built-in is refused with its
// line.
func TestProvidersFromConfig(t *testing.T) {
	dir := authTest(t)
	path := writeUserConfig(t, dir, `
[provider.zeta]
base_url = "http://localhost:1/v1"

[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := providers(cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	if strings.Join(names, " ") != "anthropic openai omniroute zeta" {
		t.Fatalf("providers %v", names)
	}

	path = writeUserConfig(t, dir, `
[chat]
usage = true

[provider.openai]
base_url = "http://localhost:20128/v1"
`)
	if cfg, err = config.Load(path); err != nil {
		t.Fatal(err)
	}
	_, err = providers(cfg, path)
	var ce *config.Error
	if !errors.As(err, &ce) || ce.Line != 5 || !strings.Contains(err.Error(), "built-in provider") {
		t.Fatalf("reserved name: %v", err)
	}
}

// TestChatOptionsEndpoints: an endpoint is in the registry and the model
// picker, its key variable is hidden from commands, and one that takes no
// key counts as connected.
func TestChatOptionsEndpoints(t *testing.T) {
	dir := authTest(t)
	t.Setenv("GW_API_KEY", "")
	path := writeUserConfig(t, dir, `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true

[provider.gw]
base_url = "https://gateway.example/v1"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	o, err := chatOptions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(o.HideEnv, "OMNIROUTE_API_KEY") || !slices.Contains(o.HideEnv, "GW_API_KEY") {
		t.Fatalf("hidden %v", o.HideEnv)
	}
	if _, ok := o.Providers.Get("omniroute"); !ok {
		t.Fatal("omniroute is not registered")
	}
	connected := map[string]bool{}
	for _, p := range o.ProviderStatuses() {
		connected[p.Name] = p.Connected
	}
	if !connected["omniroute"] || connected["gw"] {
		t.Fatalf("connected %v", connected)
	}
}

// TestAuthEndpoints: snow auth logs in to an endpoint, reports one that
// takes no key, and refuses a key for it.
func TestAuthEndpoints(t *testing.T) {
	dir := authTest(t)
	t.Setenv("GW_API_KEY", "")
	writeUserConfig(t, dir, `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true

[provider.gw]
base_url = "`+keyServer.URL+`"
`)
	authProviders = configuredProviders
	code, out, errOut := auth(t, goodKey+"\n", "login", "gw")
	if code != 0 || strings.Contains(out+errOut, goodKey) {
		t.Fatalf("login gw: code %d, out %q, err %q", code, out, errOut)
	}
	if got, _, err := credential.New(dir, nil).Get("gw"); err != nil || got.Reveal() != goodKey {
		t.Fatalf("saved: %v", err)
	}
	code, out, _ = auth(t, "", "status")
	if code != 0 || !strings.Contains(out, "omniroute  no key needed") || !strings.Contains(out, "gw         API key") {
		t.Fatalf("status: code %d\n%s", code, out)
	}
	code, _, errOut = auth(t, goodKey+"\n", "login", "omniroute")
	if code == 0 || !strings.Contains(errOut, signin.ErrNoKeyNeeded.Error()) {
		t.Fatalf("login omniroute: code %d, err %q", code, errOut)
	}
}
