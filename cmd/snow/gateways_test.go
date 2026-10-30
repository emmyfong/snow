package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/gateway"
	"github.com/emmyfong/snow/internal/signin"
	"github.com/emmyfong/snow/pkg/api"
)

func TestGatewaySpecs(t *testing.T) {
	cfg := config.Config{Providers: map[string]config.Endpoint{
		"openrouter": {BaseURL: "https://openrouter.ai/api/v1"},
		"omniroute": {
			BaseURL: "http://localhost:20128/v1",
			Start:   []string{"npx", "omniroute"},
		},
	}}
	got := gatewaySpecs(cfg)
	want := []gateway.Spec{{Name: "omniroute", BaseURL: "http://localhost:20128/v1", Start: []string{"npx", "omniroute"}}}
	if len(got) != 1 || got[0].Name != want[0].Name || got[0].BaseURL != want[0].BaseURL ||
		len(got[0].Start) != 2 || got[0].Start[0] != "npx" || got[0].Start[1] != "omniroute" {
		t.Fatalf("specs %+v, want %+v", got, want)
	}
}

// fakeGateways reports a fixed status list.
type fakeGateways []gateway.Status

func (f fakeGateways) Statuses() []gateway.Status { return f }

func TestWithGatewayStatuses(t *testing.T) {
	models := func() []api.ProviderModels {
		return []api.ProviderModels{
			{Name: "anthropic", Connected: true},
			{Name: "omniroute", Connected: true},
			{Name: "openrouter", Connected: false, Err: "no key"},
		}
	}
	check := func(t *testing.T, in []gateway.Status, name string, want api.ProviderModels) {
		t.Helper()
		out := withGatewayStatuses(models, fakeGateways(in))()
		for _, p := range out {
			if p.Name == name {
				if p.Status != want.Status || p.Err != want.Err {
					t.Fatalf("%s = status %q, err %q; want %q, %q", name, p.Status, p.Err, want.Status, want.Err)
				}
				return
			}
		}
		t.Fatalf("%s missing from %+v", name, out)
	}
	t.Run("starting", func(t *testing.T) {
		check(t, []gateway.Status{{Name: "omniroute", State: gateway.StateStarting}},
			"omniroute", api.ProviderModels{Status: "starting", Err: "starting"})
	})
	t.Run("failed", func(t *testing.T) {
		check(t, []gateway.Status{{Name: "omniroute", State: gateway.StateFailed, LogPath: "/home/u/.snow/logs/omniroute.log"}},
			"omniroute", api.ProviderModels{Status: "failed", Err: "failed, see /home/u/.snow/logs/omniroute.log"})
	})
	t.Run("ready", func(t *testing.T) {
		check(t, []gateway.Status{{Name: "omniroute", State: gateway.StateReady}},
			"omniroute", api.ProviderModels{Status: "ready"})
	})
	t.Run("a provider's own error wins", func(t *testing.T) {
		check(t, []gateway.Status{{Name: "openrouter", State: gateway.StateStarting}},
			"openrouter", api.ProviderModels{Status: "starting", Err: "no key"})
	})
	t.Run("a gateway the list lacks is added", func(t *testing.T) {
		check(t, []gateway.Status{{Name: "newgw", State: gateway.StateReady}},
			"newgw", api.ProviderModels{Status: "ready"})
	})
}

// TestAuthStatusGateway: snow auth status asks a configured gateway whether
// it answers now, and says so on the provider's line.
func TestAuthStatusGateway(t *testing.T) {
	dir := authTest(t)
	writeUserConfig(t, dir, `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true
start = ["npx", "omniroute"]
`)
	old := authGateways
	t.Cleanup(func() { authGateways = old })
	authProviders = func() ([]signin.Provider, error) {
		return []signin.Provider{{Name: "omniroute", Title: "OmniRoute", NoKey: true}}, nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	authGateways = func() []gateway.Spec {
		return []gateway.Spec{{Name: "omniroute", BaseURL: srv.URL, Start: []string{"npx", "omniroute"}}}
	}
	code, out, errOut := auth(t, "", "status")
	if code != 0 || !strings.Contains(out, "omniroute  no key needed; gateway ready") {
		t.Fatalf("ready: code %d, out %q, err %q", code, out, errOut)
	}
	srv.Close()

	code, out, errOut = auth(t, "", "status")
	if code != 0 || !strings.Contains(out, "gateway not answering yet; Snow starts it with the server") {
		t.Fatalf("not running: code %d, out %q, err %q", code, out, errOut)
	}
}

// TestGatewayText: a probe that fails for another reason still says the
// gateway does not answer.
func TestGatewayText(t *testing.T) {
	g := gateway.Spec{Name: "gw", BaseURL: "http://" + closedAddr(t) + "/v1"}
	if got := gatewayText(g); !strings.HasPrefix(got, "not answering yet") {
		t.Fatalf("gatewayText = %q", got)
	}
}

// closedAddr returns an address on this machine that nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	return addr
}
