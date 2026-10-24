package providertest

import (
	"errors"
	"io"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
)

// Key is a provider.KeySource that gives the same key for every provider.
type Key string

// APIKey implements provider.KeySource.
func (k Key) APIKey(string) (string, error) { return string(k), nil }

// Collect reads s to its end, closes it, and returns the events and the
// error that ended it: nil for io.EOF.
func Collect(t testing.TB, s provider.Stream) ([]provider.Event, error) {
	t.Helper()
	defer func() { _ = s.Close() }()
	var evs []provider.Event
	for {
		ev, err := s.Next()
		if errors.Is(err, io.EOF) {
			return evs, nil
		}
		if err != nil {
			return evs, err
		}
		evs = append(evs, ev)
	}
}

// Only returns the events of type T, in order.
func Only[T provider.Event](evs []provider.Event) []T {
	var out []T
	for _, e := range evs {
		if v, ok := e.(T); ok {
			out = append(out, v)
		}
	}
	return out
}
