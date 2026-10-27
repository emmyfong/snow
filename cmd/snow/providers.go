package main

import (
	"fmt"
	"slices"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/signin"
)

// providers lists the built-in providers, then one per [provider.<name>]
// table of cfg, in name order. A table named like a built-in provider is
// an error that names its line in the file at path.
func providers(cfg config.Config, path string) ([]signin.Provider, error) {
	ps := signin.Providers()
	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	slices.Sort(names)
	eps := make([]signin.Endpoint, 0, len(names))
	for _, name := range names {
		if _, err := signin.Find(ps, name); err == nil {
			return nil, config.Locate(path, []string{"provider", name},
				fmt.Sprintf("%q is a built-in provider; give the table another name", name), nil)
		}
		e := cfg.Providers[name]
		eps = append(eps, signin.Endpoint{Name: name, BaseURL: e.BaseURL, Models: e.Models, NoKey: e.NoKey})
	}
	return append(ps, signin.Endpoints(eps)...), nil
}

// configuredProviders is providers for the user's config file.
func configuredProviders() ([]signin.Provider, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return providers(cfg, path)
}
