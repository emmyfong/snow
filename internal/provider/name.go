package provider

import (
	"errors"
	"fmt"
	"strings"
)

// ModelName names a model across providers: "anthropic/claude-opus-5-5".
type ModelName struct {
	Provider string
	Model    string
}

// ErrBadModelName means a model name is not "provider/model".
var ErrBadModelName = errors.New("a model name is provider/model")

// ParseModelName splits s at its first slash. The model part may hold more
// slashes, as some vendors' IDs do.
func ParseModelName(s string) (ModelName, error) {
	p, m, ok := strings.Cut(s, "/")
	if !ok || p == "" || m == "" {
		return ModelName{}, fmt.Errorf("%w: %q", ErrBadModelName, s)
	}
	return ModelName{Provider: p, Model: m}, nil
}

// String gives the full name, for example "anthropic/claude-opus-5-5".
func (n ModelName) String() string { return n.Provider + "/" + n.Model }

// Undated returns a model ID without a trailing "-YYYYMMDD" snapshot date,
// for example claude-opus-5-5 for claude-opus-5-5-20261001. An ID without
// a date comes back unchanged.
func Undated(id string) string {
	const n = len("-20060102")
	if len(id) <= n || id[len(id)-n] != '-' {
		return id
	}
	for _, c := range id[len(id)-n+1:] {
		if c < '0' || c > '9' {
			return id
		}
	}
	return id[:len(id)-n]
}
