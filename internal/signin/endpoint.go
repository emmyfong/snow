package signin

import (
	"context"
	"errors"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/openai"
)

// ErrNoKeyNeeded is the cause of an error for a key given to a provider
// that takes none.
var ErrNoKeyNeeded = errors.New("this provider takes no key")

// Endpoints returns a Provider for each service that speaks the OpenAI
// API, from the [provider.<name>] tables of config.toml, in order. Its key
// is saved under the service's name, so `snow auth login <name>` and
// <NAME>_API_KEY work as for a built-in provider.
func Endpoints(cs []openai.Compat) []Provider {
	out := make([]Provider, len(cs))
	for i, c := range cs {
		out[i] = Provider{
			Name: c.Name, Title: c.Name, NoKey: c.NoKey,
			check: func(ctx context.Context, key string) error {
				return openai.NewCompatible(c, fixedKey(key)).CheckKey(ctx)
			},
			open: func(keys provider.KeySource) provider.Provider { return openai.NewCompatible(c, keys) },
		}
	}
	return out
}
