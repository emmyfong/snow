package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalidAuth is returned by the auth request Validate methods.
var ErrInvalidAuth = errors.New("api: invalid auth request")

// AuthListRequest asks the server which providers can be connected, and
// whether each one has a key.
type AuthListRequest struct{}

// AuthList is the server's answer to AuthListRequest: one entry per provider,
// in the order the picker shows them.
type AuthList struct {
	Providers []ProviderAuth `json:"providers"`
}

// ProviderAuth is one provider and the state of its API key.
type ProviderAuth struct {
	Name   string `json:"name"` // the provider's part of a model name, for example "anthropic"
	Title  string `json:"title,omitempty"`
	KeyURL string `json:"keyURL,omitempty"` // the vendor's page where the user creates a key
	// Source is where the key comes from, in words for people, for example
	// "environment" or "OS credential store". Empty when no key is saved.
	Source string `json:"source,omitempty"`
	// EnvVar is the environment variable that holds the key, when one does.
	// Snow cannot remove such a key.
	EnvVar string `json:"envVar,omitempty"`
	// Connected says a key is available to use.
	Connected bool `json:"connected,omitempty"`
	// NoKey says the provider takes no key, such as a local server. It is
	// always connected, and there is nothing to log in or out of.
	NoKey bool `json:"noKey,omitempty"`
	// Err is why the key could not be read, for example a permissions
	// error. A provider with Err set is not connected.
	Err string `json:"err,omitempty"`
}

// LoginRequest asks the server to check a provider's API key and save it
// when the provider accepts it. The key travels once, to the server; it
// never comes back and never reaches a log line.
type LoginRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

// Format prints r without its key, so a log line cannot leak it. Every verb
// goes through this method; only %T and %p show anything else, and neither
// prints a value.
func (r LoginRequest) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprintf(f, "api.LoginRequest{Provider:%q, Key:[hidden]}", r.Provider)
}

// MarshalJSON writes Providers as [] when it is empty, so a client can range
// over it without a nil check.
func (l AuthList) MarshalJSON() ([]byte, error) {
	type wire AuthList
	if l.Providers == nil {
		l.Providers = []ProviderAuth{}
	}
	return json.Marshal(wire(l))
}

// Validate checks that r names a provider and carries a key. The server
// still checks the key's shape before it saves it.
func (r *LoginRequest) Validate() error {
	if r.Provider == "" {
		return fmt.Errorf("%w: no provider", ErrInvalidAuth)
	}
	if r.Key == "" {
		return fmt.Errorf("%w: no key", ErrInvalidAuth)
	}
	return nil
}

// LoginResult says what a successful login did.
type LoginResult struct {
	// Source is where the key went, in words for people, for example "OS
	// credential store" or "credentials file".
	Source string `json:"source,omitempty"`
	// Shadowed says an environment variable holds a key for the provider,
	// and Snow uses that one first.
	Shadowed bool `json:"shadowed,omitempty"`
	// Notice is a one-time message the user should see, for example that
	// the fallback credentials file is in use.
	Notice string `json:"notice,omitempty"`
}

// Login is the server's answer to LoginRequest: the provider accepted the
// key and the server saved it.
type Login struct {
	Provider string `json:"provider"`
	LoginResult
}

// LogoutRequest asks the server to remove a provider's saved key.
type LogoutRequest struct {
	Provider string `json:"provider"`
}

// Validate checks that r names a provider.
func (r *LogoutRequest) Validate() error {
	if r.Provider == "" {
		return fmt.Errorf("%w: no provider", ErrInvalidAuth)
	}
	return nil
}

// Logout is the server's answer to LogoutRequest: the saved key is gone.
type Logout struct {
	Provider string `json:"provider"`
}

// Type implements Message.
func (*AuthListRequest) Type() MsgType { return TypeAuthListRequest }
func (*AuthListRequest) message()      {}

// Type implements Message.
func (*AuthList) Type() MsgType { return TypeAuthList }
func (*AuthList) message()      {}

// Type implements Message.
func (*LoginRequest) Type() MsgType { return TypeLoginRequest }
func (*LoginRequest) message()      {}

// Type implements Message.
func (*Login) Type() MsgType { return TypeLogin }
func (*Login) message()      {}

// Type implements Message.
func (*LogoutRequest) Type() MsgType { return TypeLogoutRequest }
func (*LogoutRequest) message()      {}

// Type implements Message.
func (*Logout) Type() MsgType { return TypeLogout }
func (*Logout) message()      {}
