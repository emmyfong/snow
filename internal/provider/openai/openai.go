// Package openai is the provider for OpenAI's Responses API, by API key. It
// streams replies with reasoning summaries and tool calls, lists models,
// and checks a key. It reads the key through provider.KeySource on every
// request and never puts it in an error.
package openai

import (
	"context"
	"net/http"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providerhttp"
)

// Name is the provider's part of a model name: "openai/gpt-6.1-sol".
const Name = "openai"

// defaultBaseURL is the API's address. The SDK reads OPENAI_BASE_URL by
// default; Snow names the address itself so a saved key never follows it.
const defaultBaseURL = "https://api.openai.com/v1/"

// Provider talks to the OpenAI API. It is safe for concurrent use.
type Provider struct {
	keys    provider.KeySource
	baseURL string
	http    *http.Client
}

// Option changes how New builds a Provider.
type Option func(*Provider)

// WithBaseURL sends requests to url instead of the OpenAI API, for tests.
func WithBaseURL(url string) Option { return func(p *Provider) { p.baseURL = url } }

// WithHTTPClient sends requests through a copy of c that refuses redirects
// to another host.
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.http = c } }

// New returns a Provider that reads its key from keys.
func New(keys provider.KeySource, opts ...Option) *Provider {
	p := &Provider{keys: keys, baseURL: defaultBaseURL}
	for _, o := range opts {
		o(p)
	}
	p.http = providerhttp.Guarded(p.http)
	return p
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// options builds the SDK's options with the current key. The services are
// built from these alone, never through sdk.NewClient: that reads
// OPENAI_BASE_URL, OPENAI_ORG_ID, OPENAI_PROJECT_ID, and
// OPENAI_CUSTOM_HEADERS, which could send the key elsewhere or bill
// another project. The SDK's retries are off too: a rate limit or an
// overload goes back to the user with its retry time, and Snow never
// retries or switches models by itself.
func (p *Provider) options() ([]option.RequestOption, error) {
	key, err := p.keys.APIKey(Name)
	if err != nil {
		return nil, &provider.Error{Kind: provider.KindNoKey, Err: err}
	}
	return []option.RequestOption{
		option.WithHTTPClient(p.http),
		option.WithBaseURL(p.baseURL),
		option.WithAPIKey(key),
		option.WithMaxRetries(0),
	}, nil
}

// CheckKey makes one model list call: nil means the key works, and an
// error of KindBadKey means the API rejected it.
func (p *Provider) CheckKey(ctx context.Context) error {
	opts, err := p.options()
	if err != nil {
		return err
	}
	models := sdk.NewModelService(opts...)
	if _, err := models.List(ctx); err != nil {
		return mapErr(ctx, err)
	}
	return nil
}

// Stream implements provider.Provider. Reasoning and effort are sent only
// to a model that takes them.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	opts, err := p.options()
	if err != nil {
		return nil, err
	}
	params, err := toParams(req, capsFor(req.Model))
	if err != nil {
		return nil, err
	}
	svc := responses.NewResponseService(opts...)
	return newStream(ctx, svc.NewStreaming(ctx, params)), nil
}
