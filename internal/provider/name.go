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

// Undated returns a model ID without a trailing snapshot date, for example
// claude-opus-5-5 for claude-opus-5-5-20261001 and gpt-5.5 for
// gpt-5.5-2026-04-23. An ID without a date comes back unchanged.
func Undated(id string) string {
	for _, layout := range []string{"20060102", "2006-01-02"} {
		n := len(layout) + 1
		if len(id) <= n || id[len(id)-n] != '-' {
			continue
		}
		if datePattern(id[len(id)-n+1:], layout) {
			return id[:len(id)-n]
		}
	}
	return id
}

// datePattern reports whether s has a digit wherever layout has one and
// the same character elsewhere.
func datePattern(s, layout string) bool {
	for i := range len(layout) {
		c, l := s[i], layout[i]
		if l >= '0' && l <= '9' {
			if c < '0' || c > '9' {
				return false
			}
		} else if c != l {
			return false
		}
	}
	return true
}
