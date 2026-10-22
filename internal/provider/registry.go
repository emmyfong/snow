package provider

import (
	"errors"
	"fmt"
	"sort"
)

// ErrDuplicate means a provider with that name is already registered.
var ErrDuplicate = errors.New("provider already registered")

// Registry finds providers by name. The server fills one at startup.
type Registry struct{ byName map[string]Provider }

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{byName: map[string]Provider{}} }

// Register adds p under p.Name().
func (r *Registry) Register(p Provider) error {
	if _, ok := r.byName[p.Name()]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicate, p.Name())
	}
	r.byName[p.Name()] = p
	return nil
}

// Get returns the provider called name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.byName[name]
	return p, ok
}

// Names lists the registered providers in order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
