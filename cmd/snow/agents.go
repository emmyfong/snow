package main

import (
	"fmt"
	"maps"
	"slices"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/signin"
)

// agentProfiles returns the [agent.<name>] tables of cfg for the server,
// in name order, and the secrets they name that no provider in ps holds:
// tokens such as a gateway's, which `snow auth login <name>` saves. A
// secret that names a provider with no_key = true is an error at its line
// in the file at path.
func agentProfiles(cfg config.Config, path string, ps []signin.Provider) ([]server.AgentProfile, []string, error) {
	var (
		out    []server.AgentProfile
		tokens []string
	)
	for _, name := range slices.Sorted(maps.Keys(cfg.Agents)) {
		a := cfg.Agents[name]
		p := server.AgentProfile{Name: a.TitleOf(name), Command: slices.Clone(a.Command)}
		for _, v := range slices.Sorted(maps.Keys(a.Env)) {
			secret, ok := config.SecretName(a.Env[v])
			if !ok {
				p.Env = append(p.Env, server.EnvVar{Name: v, Value: a.Env[v]})
				continue
			}
			p.Env = append(p.Env, server.EnvVar{Name: v, Secret: secret})
			switch found, err := signin.Find(ps, secret); {
			case err != nil:
				if !slices.Contains(tokens, secret) {
					tokens = append(tokens, secret)
				}
			case found.NoKey:
				return nil, nil, config.Locate(path, []string{"agent", name, "env", v},
					fmt.Sprintf("names the secret %q, but [provider.%s] has no_key = true, so no key is saved for it", secret, secret), nil)
			}
		}
		out = append(out, p)
	}
	slices.Sort(tokens)
	return out, tokens, nil
}

// agentSetup is what the server needs for agent profiles.
type agentSetup struct {
	profiles []server.AgentProfile
	secrets  server.SecretSource
	keyVars  []string // the variables of every provider key and token
}

// agentOptions builds the server's agent profiles for cfg, with the
// credential store their secrets come from.
func agentOptions(cfg config.Config) (agentSetup, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return agentSetup{}, err
	}
	ps, err := providers(cfg, path)
	if err != nil {
		return agentSetup{}, err
	}
	profiles, tokens, err := agentProfiles(cfg, path, ps)
	if err != nil {
		return agentSetup{}, err
	}
	var keyVars []string
	for _, p := range append(ps, signin.Tokens(tokens)...) {
		keyVars = append(keyVars, credential.EnvVar(p.Name))
	}
	setup := agentSetup{profiles: profiles, keyVars: keyVars}
	dir, err := credential.DefaultDir()
	if err != nil {
		// The profiles still run; those that name a secret say why not.
		return setup, err
	}
	setup.secrets = credential.New(dir, nil)
	return setup, nil
}
