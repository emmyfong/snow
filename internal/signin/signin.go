// Package signin connects Snow to providers: it checks an API key with its
// provider and saves it, removes a saved key, and reports which providers
// have one. The `snow auth` commands use it, and /login in the chat will.
package signin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/anthropic"
	"github.com/emmyfong/snow/internal/provider/openai"
)

// Store keeps keys per provider. credential.Store satisfies it.
type Store interface {
	Get(provider string) (credential.Key, credential.Source, error)
	Set(provider string, k credential.Key) (credential.Source, error)
	Delete(provider string) error
}

// Provider is a provider the user can connect.
type Provider struct {
	Name   string // the provider's part of a model name, for example "anthropic"
	Title  string // the name to show, for example "Anthropic (Claude)"
	KeyURL string // the vendor's page where the user creates a key
	// DefaultModel is the model a new chat uses, as "provider/model", when
	// the user chose none.
	DefaultModel string
	check        func(ctx context.Context, key string) error
	open         func(keys provider.KeySource) provider.Provider
}

// ErrUnknownProvider means no provider has the name given.
var ErrUnknownProvider = errors.New("unknown provider")

// fixedKey gives a provider the key being checked, before it is saved.
type fixedKey string

func (k fixedKey) APIKey(string) (string, error) { return string(k), nil }

// BaseURLs points the providers at other servers, for tests. An empty
// field keeps the vendor's API.
type BaseURLs struct{ Anthropic, OpenAI string }

// Providers lists the providers Snow can connect, in a fixed order.
func Providers() []Provider { return ProvidersAt(BaseURLs{}) }

// ProvidersAt is Providers with the key checks sent to urls.
func ProvidersAt(urls BaseURLs) []Provider {
	return []Provider{
		{
			Name: anthropic.Name, Title: "Anthropic (Claude)", KeyURL: "https://platform.claude.com/settings/keys",
			DefaultModel: anthropic.Name + "/claude-sonnet-5-5",
			check: func(ctx context.Context, key string) error {
				return anthropic.New(fixedKey(key), anthropicOpts(urls)...).CheckKey(ctx)
			},
			open: func(keys provider.KeySource) provider.Provider { return anthropic.New(keys, anthropicOpts(urls)...) },
		},
		{
			Name: openai.Name, Title: "OpenAI", KeyURL: "https://platform.openai.com/api-keys",
			DefaultModel: openai.Name + "/gpt-6.1-sol",
			check: func(ctx context.Context, key string) error {
				return openai.New(fixedKey(key), openaiOpts(urls)...).CheckKey(ctx)
			},
			open: func(keys provider.KeySource) provider.Provider { return openai.New(keys, openaiOpts(urls)...) },
		},
	}
}

func anthropicOpts(urls BaseURLs) []anthropic.Option {
	if urls.Anthropic == "" {
		return nil
	}
	return []anthropic.Option{anthropic.WithBaseURL(urls.Anthropic)}
}

func openaiOpts(urls BaseURLs) []openai.Option {
	if urls.OpenAI == "" {
		return nil
	}
	return []openai.Option{openai.WithBaseURL(urls.OpenAI)}
}

// Registry returns the providers of ps, ready to stream, with their keys
// read from keys when a request needs one.
func Registry(ps []Provider, keys provider.KeySource) (*provider.Registry, error) {
	reg := provider.NewRegistry()
	for _, p := range ps {
		if err := reg.Register(p.open(keys)); err != nil {
			return nil, fmt.Errorf("connect %s: %w", p.Name, err)
		}
	}
	return reg, nil
}

// DefaultModel returns the default model of the first provider in ps with
// a key in s, or of the first provider when none has one: its first turn
// then asks the user to connect it.
func DefaultModel(ps []Provider, s Store) string {
	for _, st := range Statuses(ps, s) {
		if st.Connected() {
			return st.Provider.DefaultModel
		}
	}
	if len(ps) == 0 {
		return ""
	}
	return ps[0].DefaultModel
}

// Find returns the provider in ps called name.
func Find(ps []Provider, name string) (Provider, error) {
	names := make([]string, len(ps))
	for i, p := range ps {
		if p.Name == name {
			return p, nil
		}
		names[i] = p.Name
	}
	return Provider{}, fmt.Errorf("%w %q; use one of: %s", ErrUnknownProvider, name, strings.Join(names, ", "))
}

// Result is what Login did.
type Result struct {
	Source   credential.Source // where the key went
	Shadowed bool              // an environment variable holds a key for the provider, and Snow uses it first
}

// Login checks k with the provider and saves it only if the provider
// accepts it. A rejected key comes back as a *provider.Error of
// KindBadKey; a check that could not run (no network, a timeout, an
// overload) saves nothing either and says so.
func (p Provider) Login(ctx context.Context, s Store, k credential.Key) (Result, error) {
	if p.check == nil {
		return Result{}, fmt.Errorf("%w %q", ErrUnknownProvider, p.Name)
	}
	if err := p.check(ctx, k.Reveal()); err != nil {
		var pe *provider.Error
		if errors.As(err, &pe) && pe.Kind == provider.KindBadKey {
			return Result{}, fmt.Errorf("%s did not accept the key: %w", p.Title, err)
		}
		return Result{}, fmt.Errorf("could not check the key with %s, so it was not saved: %w", p.Title, err)
	}
	src, err := s.Set(p.Name, k)
	if err != nil {
		return Result{Source: src}, fmt.Errorf("save the key: %w", err)
	}
	_, now, err := s.Get(p.Name)
	return Result{Source: src, Shadowed: err == nil && now == credential.SourceEnv}, nil
}

// Stored reports whether a key is saved for the provider, even one that
// cannot be read, so Logout can remove it. It returns an error matching
// credential.ErrInEnvironment when an environment variable holds the key,
// which Snow cannot remove.
func (p Provider) Stored(s Store) (bool, error) {
	_, src, err := s.Get(p.Name)
	switch {
	case err == nil && src == credential.SourceEnv:
		return false, fmt.Errorf("%w (%s)", credential.ErrInEnvironment, credential.EnvVar(p.Name))
	case errors.Is(err, credential.ErrNoKey):
		return false, nil
	default:
		return true, nil
	}
}

// Logout removes the provider's saved key. It returns an error matching
// credential.ErrInEnvironment when the key comes from an environment
// variable, and credential.ErrNoKey when nothing is saved.
func (p Provider) Logout(s Store) error { return s.Delete(p.Name) }

// Status is whether a provider has a key, and from where.
type Status struct {
	Provider Provider
	Source   credential.Source // 0 when there is no key
	Err      error             // nil, or why the key could not be read; ErrNoKey when none is saved
}

// Connected reports whether the provider has a key.
func (s Status) Connected() bool { return s.Err == nil }

// Statuses reports the key of every provider in ps, without reading out
// any key.
func Statuses(ps []Provider, s Store) []Status {
	out := make([]Status, len(ps))
	for i, p := range ps {
		_, src, err := s.Get(p.Name)
		out[i] = Status{Provider: p, Source: src, Err: err}
	}
	return out
}
