// Package anthropic is the provider for Anthropic's Messages API, by API
// key. It streams replies with thinking and tool use, lists models, and
// checks a key. It reads the key through provider.KeySource on every
// request and never puts it in an error.
package anthropic

import (
	"context"
	"fmt"
	"net/http"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/emmyfong/snow/internal/provider"
)

// Name is the provider's part of a model name: "anthropic/claude-opus-5-5".
const Name = "anthropic"

// defaultMaxTokens leaves room for long replies; streaming keeps a large
// limit from timing out.
const defaultMaxTokens = 64000

// Provider talks to the Anthropic API.
type Provider struct {
	keys    provider.KeySource
	baseURL string
	client  *http.Client
}

// Option changes how New builds a Provider.
type Option func(*Provider)

// WithBaseURL sends requests to url instead of the Anthropic API, for tests.
func WithBaseURL(url string) Option { return func(p *Provider) { p.baseURL = url } }

// WithHTTPClient sends requests through c.
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.client = c } }

// New returns a Provider that reads its key from keys.
func New(keys provider.KeySource, opts ...Option) *Provider {
	p := &Provider{keys: keys}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// sdkClient builds a client with the current key. The SDK's own retries
// are off: a rate limit or an overload goes back to the user with its
// retry time, and Snow never retries or switches models by itself.
func (p *Provider) sdkClient() (sdk.Client, error) {
	key, err := p.keys.APIKey(Name)
	if err != nil {
		return sdk.Client{}, fmt.Errorf("anthropic: %w", err)
	}
	opts := []option.RequestOption{option.WithAPIKey(key), option.WithMaxRetries(0)}
	if p.baseURL != "" {
		opts = append(opts, option.WithBaseURL(p.baseURL))
	}
	if p.client != nil {
		opts = append(opts, option.WithHTTPClient(p.client))
	}
	return sdk.NewClient(opts...), nil
}

// Models implements provider.Provider. Retired models are left out.
func (p *Provider) Models(ctx context.Context) ([]provider.Model, error) {
	c, err := p.sdkClient()
	if err != nil {
		return nil, err
	}
	var out []provider.Model
	pager := c.Models.ListAutoPaging(ctx, sdk.ModelListParams{})
	for pager.Next() {
		m := pager.Current()
		if m.Lifecycle == sdk.ModelInfoLifecycleRetired {
			continue
		}
		out = append(out, provider.Model{ID: m.ID, DisplayName: m.DisplayName, ContextWindow: int(m.MaxInputTokens)})
	}
	if err := pager.Err(); err != nil {
		return nil, mapErr(ctx, err)
	}
	return out, nil
}

// CheckKey makes one small model list call: nil means the key works, and
// an error of KindBadKey means the API rejected it.
func (p *Provider) CheckKey(ctx context.Context) error {
	c, err := p.sdkClient()
	if err != nil {
		return err
	}
	if _, err := c.Models.List(ctx, sdk.ModelListParams{Limit: sdk.Int(1)}); err != nil {
		return mapErr(ctx, err)
	}
	return nil
}

// Stream implements provider.Provider.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	params, err := toParams(req)
	if err != nil {
		return nil, err
	}
	c, err := p.sdkClient()
	if err != nil {
		return nil, err
	}
	return newStream(ctx, c.Messages.NewStreaming(ctx, params)), nil
}
