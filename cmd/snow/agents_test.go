package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/server"
)

// TestAgentProfilesFromConfig: the profiles come in table order with their
// titles, a "$secret:" value becomes a secret, and only a secret that no
// provider holds becomes a token.
func TestAgentProfilesFromConfig(t *testing.T) {
	dir := authTest(t)
	path := writeUserConfig(t, dir, `
[provider.omniroute]
base_url = "http://localhost:20128/v1"

[agent.claude-free]
title = "Claude Code (free)"
command = ["claude"]
env = { ANTHROPIC_BASE_URL = "http://localhost:20128", ANTHROPIC_AUTH_TOKEN = "$secret:omniroute" }

[agent.aider]
command = ["aider"]
env = { GW_TOKEN = "$secret:gw" }
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := providers(cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	profiles, tokens, err := agentProfiles(cfg, path, ps)
	if err != nil {
		t.Fatal(err)
	}
	want := []server.AgentProfile{
		{Name: "aider", Command: []string{"aider"}, Env: []server.EnvVar{{Name: "GW_TOKEN", Secret: "gw"}}},
		{Name: "Claude Code (free)", Command: []string{"claude"}, Env: []server.EnvVar{
			{Name: "ANTHROPIC_AUTH_TOKEN", Secret: "omniroute"},
			{Name: "ANTHROPIC_BASE_URL", Value: "http://localhost:20128"},
		}},
	}
	if !reflect.DeepEqual(profiles, want) {
		t.Fatalf("profiles %+v, want %+v", profiles, want)
	}
	if !reflect.DeepEqual(tokens, []string{"gw"}) {
		t.Fatalf("tokens %q, want [gw]", tokens)
	}
}

// TestLoadConfigSecretOfNoKeyProvider: a secret named after a provider
// that takes no key stops snow with the file and the line.
func TestLoadConfigSecretOfNoKeyProvider(t *testing.T) {
	dir := authTest(t)
	path := writeUserConfig(t, dir, `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true

[agent.claude-free]
command = ["claude"]

[agent.claude-free.env]
ANTHROPIC_BASE_URL = "http://localhost:20128"
ANTHROPIC_AUTH_TOKEN = "$secret:omniroute"
`)
	var stderr strings.Builder
	if _, ok := loadConfig(&stderr); ok {
		t.Fatal("loadConfig accepted a secret no key is saved for")
	}
	if got := stderr.String(); !strings.Contains(got, path) || !strings.Contains(got, "line 11") || !strings.Contains(got, "no_key") {
		t.Fatalf("stderr %q", got)
	}
}

// TestAuthLoginToken: `snow auth login <name>` saves a token for a secret
// that no provider holds, unchecked, and status lists it.
func TestAuthLoginToken(t *testing.T) {
	dir := authTest(t)
	t.Setenv("OMNIROUTE_API_KEY", "")
	writeUserConfig(t, dir, `
[agent.claude-free]
command = ["claude"]
env = { ANTHROPIC_BASE_URL = "http://localhost:20128", ANTHROPIC_AUTH_TOKEN = "$secret:omniroute" }
`)
	authProviders = configuredProviders
	code, out, errOut := auth(t, goodKey+"\n", "login", "omniroute")
	if code != 0 || !strings.Contains(out, "Saved the omniroute token in the OS credential store.") || strings.Contains(out, "Checking") {
		t.Fatalf("login: code %d, out %q, err %q", code, out, errOut)
	}
	k, _, err := credential.New(dir, nil).Get("omniroute")
	if err != nil || k.Reveal() != goodKey {
		t.Fatalf("saved token: %v", err)
	}
	if _, out, _ = auth(t, "", "status"); !strings.Contains(out, "omniroute  token for agent profiles (OS credential store)") {
		t.Fatalf("status %q", out)
	}
	if code, out, _ = auth(t, "y\n", "logout", "omniroute"); code != 0 || !strings.Contains(out, "Removed the omniroute token.") {
		t.Fatalf("logout: code %d, out %q", code, out)
	}
	if _, _, err := credential.New(dir, nil).Get("omniroute"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("token still saved: %v", err)
	}
}
