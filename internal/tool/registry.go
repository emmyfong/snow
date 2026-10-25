package tool

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrDuplicate is the cause of an error when a tool's name is already
// registered.
var ErrDuplicate = errors.New("tool name is already registered")

// Registry finds tools by name.
type Registry struct{ byName map[string]Tool }

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{byName: map[string]Tool{}} }

// builtin holds the tools that register themselves in init.
var builtin = NewRegistry()

// register adds t to the built-in registry. A duplicate name is a bug in
// this package, so it panics.
func register(t Tool) {
	if err := builtin.Register(t); err != nil {
		panic(err)
	}
}

// Builtin returns a new registry with every built-in tool. The caller may
// add more without changing the built-in set.
func Builtin() *Registry {
	return &Registry{byName: maps.Clone(builtin.byName)}
}

// Register adds t. A name that is taken gives an error wrapping
// ErrDuplicate.
func (r *Registry) Register(t Tool) error {
	name := t.Def().Name
	if _, ok := r.byName[name]; ok {
		return fmt.Errorf("register tool %q: %w", name, ErrDuplicate)
	}
	r.byName[name] = t
	return nil
}

// Get returns the tool called name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

// Defs returns the definitions of every tool, sorted by name, so requests
// list tools in the same order every time.
func (r *Registry) Defs() []Def {
	defs := make([]Def, 0, len(r.byName))
	for _, name := range slices.Sorted(maps.Keys(r.byName)) {
		defs = append(defs, r.byName[name].Def())
	}
	return defs
}
