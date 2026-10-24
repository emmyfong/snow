// Package anthropic is the provider for Anthropic's Messages API, by API
// key. It streams replies with thinking and tool use, lists models, and
// checks a key. It reads the key through provider.KeySource on every
// request and never puts it in an error.
package anthropic

import (
	"context"
	"errors"
	"net/http"
	"sync"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providerhttp"
)

// Name is the provider's part of a model name: "anthropic/claude-opus-5-5".
const Name = "anthropic"

// defaultMaxTokens leaves room for long replies; streaming keeps a large
// limit from timing out.
const defaultMaxTokens = 64000

// Provider talks to the Anthropic API. It is safe for concurrent use.
type Provider struct {
	keys    provider.KeySource
	baseURL string
	http    *http.Client

	mu     sync.Mutex
	caps   map[string]caps // by model ID, filled by Models
	listed bool            // Models succeeded once
}

// Option changes how New builds a Provider.
type Option func(*Provider)

// WithBaseURL sends requests to url instead of the Anthropic API, for tests.
func WithBaseURL(url string) Option { return func(p *Provider) { p.baseURL = url } }

// WithHTTPClient sends requests through a copy of c that refuses redirects
// to another host.
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.http = c } }

// New returns a Provider that reads its key from keys.
func New(keys provider.KeySource, opts ...Option) *Provider {
	p := &Provider{keys: keys, caps: map[string]caps{}}
	for _, o := range opts {
		o(p)
	}
	p.http = providerhttp.Guarded(p.http)
	return p
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// sdkClient builds a client with the current key. The SDK reads
// ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, and profile files by default;
// those are off, so a key Snow saved goes only to the Anthropic API. The
// SDK's retries are off too: a rate limit or an overload goes back to the
// user with its retry time, and Snow never retries or switches models by
// itself.
func (p *Provider) sdkClient() (sdk.Client, error) {
	key, err := p.keys.APIKey(Name)
	if err != nil {
		return sdk.Client{}, &provider.Error{Kind: provider.KindNoKey, Err: err}
	}
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(key),
		option.WithMaxRetries(0),
		option.WithHTTPClient(p.http),
	}
	if p.baseURL != "" {
		opts = append(opts, option.WithBaseURL(p.baseURL))
	}
	return sdk.NewClient(opts...), nil
}

// CheckKey makes one small model list call: nil means the key works, and
// an error of KindBadKey means the API rejected it. A 403 passes: the key
// is valid but may lack access to the model list.
func (p *Provider) CheckKey(ctx context.Context) error {
	c, err := p.sdkClient()
	if err != nil {
		return err
	}
	if _, err := c.Models.List(ctx, sdk.ModelListParams{Limit: sdk.Int(1)}); err != nil {
		var apiErr *sdk.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return nil
		}
		return mapErr(ctx, err)
	}
	return nil
}

// Stream implements provider.Provider. Thinking and effort are sent only
// when the model takes them; the first request for a model the provider
// has not seen lists the models to find out.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	c, err := p.sdkClient()
	if err != nil {
		return nil, err
	}
	mc, err := p.capsFor(ctx, req.Model)
	if err != nil {
		return nil, err
	}
	params, err := toParams(req, mc)
	if err != nil {
		return nil, err
	}
	return newStream(ctx, c.Messages.NewStreaming(ctx, params)), nil
}
