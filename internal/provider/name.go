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

func (n ModelName) String() string { return n.Provider + "/" + n.Model }
