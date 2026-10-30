package main

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/gateway"
	"github.com/emmyfong/snow/pkg/api"
)

// gatewaySpecs returns the [provider.<name>] tables that set start, in name
// order.
func gatewaySpecs(cfg config.Config) []gateway.Spec {
	var out []gateway.Spec
	for _, name := range slices.Sorted(maps.Keys(cfg.Providers)) {
		p := cfg.Providers[name]
		if len(p.Start) == 0 {
			continue
		}
		out = append(out, gateway.Spec{Name: name, BaseURL: p.BaseURL, Start: slices.Clone(p.Start)})
	}
	return out
}

// startGateways starts the gateways Snow may start, without blocking the
// server, and returns the manager that reports and stops them. A config
// with no gateway returns nil.
func startGateways(cfg config.Config, log *slog.Logger) *gateway.Manager {
	specs := gatewaySpecs(cfg)
	if len(specs) == 0 {
		return nil
	}
	m := gateway.New(specs, gateway.Options{Log: log})
	m.Start()
	return m
}

// gatewayStates is what the model picker needs from the gateway manager.
// Tests use a fake.
type gatewayStates interface {
	Statuses() []gateway.Status
}

// withGatewayStatuses adds each gateway's state to the provider list the
// model picker sees. The gateway's detail replaces a model fetch error only
// when the provider has none of its own, such as a missing key.
func withGatewayStatuses(next func() []api.ProviderModels, gw gatewayStates) func() []api.ProviderModels {
	return func() []api.ProviderModels {
		var out []api.ProviderModels
		if next != nil {
			out = next()
		}
		for _, st := range gw.Statuses() {
			i := slices.IndexFunc(out, func(p api.ProviderModels) bool { return p.Name == st.Name })
			if i < 0 {
				out = append(out, api.ProviderModels{Name: st.Name, Title: st.Name, Connected: true})
				i = len(out) - 1
			}
			out[i].Status = st.Word()
			if st.State != gateway.StateReady && out[i].Err == "" {
				out[i].Err = st.Detail()
			}
		}
		return out
	}
}

// authGateways lists the gateways `snow auth status` probes. Tests replace
// it.
var authGateways = configuredGateways

// configuredGateways returns the gateways of the user's config file. A file
// that cannot be read gives none: `snow auth status` reports its error
// elsewhere.
func configuredGateways() []gateway.Spec {
	path, err := config.DefaultPath()
	if err != nil {
		return nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil
	}
	return gatewaySpecs(cfg)
}

// gatewayText says whether a gateway answers now, for `snow auth status`.
// That command runs in its own process, so it asks the address instead of
// reading the server's state.
func gatewayText(g gateway.Spec) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := gateway.Probe(ctx, g.BaseURL); err != nil {
		return "not answering yet; Snow starts it with the server"
	}
	return "ready"
}
