package openai

import (
	"slices"

	"github.com/emmyfong/snow/internal/provider"
)

// Compat describes a service that speaks the OpenAI API, such as
// OmniRoute, OpenRouter, or Ollama, from a [provider.<name>] table.
type Compat struct {
	// Name is the provider's part of a model name, and the name its key
	// is saved under.
	Name string
	// BaseURL is the service's API address, for example
	// "http://localhost:20128/v1".
	BaseURL string
	// Models are offered as they are, for an endpoint that cannot list
	// its own; empty asks the endpoint.
	Models []string
	// NoKey sends requests without a key, for a local server.
	NoKey bool
}

// NewCompatible returns a Provider for the service c, with its key read
// from keys under c.Name. It keeps every guard of New: no SDK environment
// defaults, no redirect to another host, no retries, and errors with no
// key in them. The service's model list is offered whole, as Snow cannot
// tell its text models apart.
func NewCompatible(c Compat, keys provider.KeySource, opts ...Option) *Provider {
	p := New(keys, append([]Option{WithBaseURL(c.BaseURL)}, opts...)...)
	p.name, p.compat, p.models, p.noKey = c.Name, true, slices.Clone(c.Models), c.NoKey
	return p
}
