// Package openai is the provider for OpenAI's Responses API, by API key. It
// streams replies with reasoning summaries and tool calls, lists models,
// and checks a key. It reads the key through provider.KeySource on every
// request and never puts it in an error.
package openai

import (
	"context"
	"errors"
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

// Provider talks to the OpenAI API, or to a service that speaks it (see
// NewCompatible). It is safe for concurrent use.
type Provider struct {
	keys    provider.KeySource
	baseURL string
	http    *http.Client
	name    string   // Name, or the compatible service's name
	compat  bool     // a compatible service: its whole model list is offered
	models  []string // a compatible service's models from the config, if any
	noKey   bool     // requests go without a key
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
	p := &Provider{keys: keys, baseURL: defaultBaseURL, name: Name}
	for _, o := range opts {
		o(p)
	}
	p.http = providerhttp.Guarded(p.http)
	return p
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return p.name }

// options builds the SDK's options with the current key. The services are
// built from these alone, never through sdk.NewClient: that reads
// OPENAI_BASE_URL, OPENAI_ORG_ID, OPENAI_PROJECT_ID, and
// OPENAI_CUSTOM_HEADERS, which could send the key elsewhere or bill
// another project. The SDK's retries are off too: a rate limit or an
// overload goes back to the user with its retry time, and Snow never
// retries or switches models by itself.
//
// It also returns the key, which errors must not repeat; it is empty for a
// service that takes none.
func (p *Provider) options() ([]option.RequestOption, string, error) {
	opts := []option.RequestOption{
		option.WithHTTPClient(p.http),
		option.WithBaseURL(p.baseURL),
		option.WithMaxRetries(0),
	}
	if p.noKey {
		// With no key set, the SDK sends no Authorization header.
		return opts, "", nil
	}
	key, err := p.keys.APIKey(p.name)
	if err != nil {
		return nil, "", &provider.Error{Kind: provider.KindNoKey, Err: err}
	}
	return append(opts, option.WithAPIKey(key)), key, nil
}

// ErrKeyNotChecked is what CheckKey returns for a compatible service with
// no model list: nothing else checks a key without spending a request, so
// the key may be saved, unchecked.
var ErrKeyNotChecked = errors.New("the service has no model list, so the key could not be checked")

// CheckKey makes one model list call: nil means the key works, and an
// error of KindBadKey means the API rejected it. For OpenAI a 403 passes:
// the key is valid, but a restricted key may not list models and can still
// reply. A compatible service's 403 fails, as gateways send it for a bad
// key, and its 404 gives ErrKeyNotChecked.
func (p *Provider) CheckKey(ctx context.Context) error {
	opts, key, err := p.options()
	if err != nil {
		return err
	}
	models := sdk.NewModelService(opts...)
	if _, err := models.List(ctx); err != nil {
		var apiErr *sdk.Error
		if errors.As(err, &apiErr) {
			switch {
			case apiErr.StatusCode == http.StatusForbidden && !p.compat:
				return nil
			case apiErr.StatusCode == http.StatusNotFound && p.compat:
				return ErrKeyNotChecked
			}
		}
		return mapErr(ctx, err, key)
	}
	return nil
}

// Stream implements provider.Provider. Reasoning and effort are sent only
// to a model that takes them.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	opts, key, err := p.options()
	if err != nil {
		return nil, err
	}
	params, err := toParams(req, capsFor(req.Model))
	if err != nil {
		return nil, err
	}
	svc := responses.NewResponseService(opts...)
	return newStream(ctx, svc.NewStreaming(ctx, params), key), nil
}
