package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// Endpoint is a provider the user adds in a [provider.<name>] table: a
// service that speaks the OpenAI API, such as OmniRoute, OpenRouter, or
// Ollama. Its models are named "<name>/<model>". Its key is never in the
// file: it comes from <NAME>_API_KEY or the credential store.
type Endpoint struct {
	// BaseURL is the API's address, for example
	// "http://localhost:20128/v1".
	BaseURL string `toml:"base_url"`
	// Models lists the model IDs to offer, for an endpoint that cannot
	// list its own. Empty asks the endpoint.
	Models []string `toml:"models"`
	// NoKey marks an endpoint that takes no key, such as a local server:
	// requests go without one.
	NoKey bool `toml:"no_key"`
	// API is the API the service speaks: APIResponses (the default when
	// empty) or APIChat. Snow never tries the other one by itself.
	API string `toml:"api"`
}

// The APIs a service can speak.
const (
	// APIResponses is the OpenAI Responses API, POST <base_url>/responses.
	APIResponses = "responses"
	// APIChat is the Chat Completions API, POST <base_url>/chat/completions,
	// which Ollama, LM Studio, llama.cpp, vLLM, and many gateways serve.
	APIChat = "chat"
)

// Protocol returns the API the endpoint speaks, with the default filled in.
func (e Endpoint) Protocol() string {
	if e.API == "" {
		return APIResponses
	}
	return e.API
}

// maxProviderName bounds a provider's name, which starts every model name
// of the provider.
const maxProviderName = 32

// validateProviders rejects endpoints that can never work. It returns the
// error and the setting's path in the file. The first problem in name
// order wins, so the answer does not change between runs.
func (c Config) validateProviders() (*Error, []string) {
	names := make([]string, 0, len(c.Providers))
	for name := range c.Providers {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		e := c.Providers[name]
		key := "provider." + quoteKey(name)
		if msg := providerName(name); msg != "" {
			return &Error{Key: key, Msg: msg}, []string{"provider", name}
		}
		if msg := baseURL(e.BaseURL); msg != "" {
			return &Error{Key: key + ".base_url", Msg: msg}, []string{"provider", name, "base_url"}
		}
		if e.API != "" && e.API != APIResponses && e.API != APIChat {
			return &Error{Key: key + ".api", Msg: fmt.Sprintf("must be %q or %q, got %q", APIResponses, APIChat, e.API)},
				[]string{"provider", name, "api"}
		}
		for _, m := range e.Models {
			if strings.TrimSpace(m) == "" || strings.ContainsAny(m, " \t\r\n") {
				return &Error{Key: key + ".models", Msg: fmt.Sprintf("model IDs must not be empty or hold spaces, got %q", m)},
					[]string{"provider", name, "models"}
			}
		}
	}
	return nil, nil
}

// providerName says what is wrong with a provider's name, or "". The name
// is the first part of a model name and of the <NAME>_API_KEY variable, so
// it is lowercase letters, digits, and dashes.
func providerName(name string) string {
	ok := name != "" && len(name) <= maxProviderName && name[0] != '-'
	for _, r := range name {
		lower, digit := r >= 'a' && r <= 'z', r >= '0' && r <= '9'
		if !lower && !digit && r != '-' {
			ok = false
		}
	}
	if !ok {
		return fmt.Sprintf("the name must be 1 to %d lowercase letters, digits, or dashes, and not start with a dash", maxProviderName)
	}
	return ""
}

// baseURL says what is wrong with an endpoint's address, or "".
func baseURL(s string) string {
	if s == "" {
		return `must be set, for example "http://localhost:20128/v1"`
	}
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return fmt.Sprintf("is not a URL: %v", err)
	case u.Scheme != "http" && u.Scheme != "https":
		return fmt.Sprintf("must start with http:// or https://, got %q", s)
	case u.Host == "":
		return fmt.Sprintf("must name a host, got %q", s)
	case u.User != nil:
		// A key in the address would sit in plain text in the file.
		return "must not hold a user name or a key; save the key with snow auth login"
	case u.RawQuery != "" || u.Fragment != "":
		return fmt.Sprintf("must not have a query or a fragment, got %q", s)
	}
	return ""
}
