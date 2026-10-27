package signin

import (
	"context"
	"errors"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/openai"
)

// Endpoint is a service that speaks the OpenAI API, from a
// [provider.<name>] table in config.toml.
type Endpoint struct {
	Name    string
	BaseURL string
	Models  []string // offered as they are; empty asks the service
	NoKey   bool     // the service takes no key
}

// ErrNoKeyNeeded is the cause of an error for a key given to a provider
// that takes none.
var ErrNoKeyNeeded = errors.New("this provider takes no key")

// Endpoints returns a Provider for each endpoint, in order. Its key is
// saved under the endpoint's name, so `snow auth login <name>` and
// <NAME>_API_KEY work as for a built-in provider.
func Endpoints(eps []Endpoint) []Provider {
	out := make([]Provider, len(eps))
	for i, e := range eps {
		c := openai.Compat{Name: e.Name, BaseURL: e.BaseURL, Models: e.Models, NoKey: e.NoKey}
		out[i] = Provider{
			Name: e.Name, Title: e.Name, NoKey: e.NoKey,
			check: func(ctx context.Context, key string) error {
				return openai.NewCompatible(c, fixedKey(key)).CheckKey(ctx)
			},
			open: func(keys provider.KeySource) provider.Provider { return openai.NewCompatible(c, keys) },
		}
	}
	return out
}
