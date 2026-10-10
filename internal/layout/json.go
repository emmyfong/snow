package layout

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON writes the tree for a snapshot.
func (t *Tree) MarshalJSON() ([]byte, error) {
	type wire Tree
	b, err := json.Marshal((*wire)(t))
	if err != nil {
		return nil, fmt.Errorf("layout: marshal tree: %w", err)
	}
	return b, nil
}

// UnmarshalJSON reads a tree written by MarshalJSON. It rejects malformed or
// invalid input with ErrInvalidTree and leaves t unchanged, so a corrupt
// snapshot cannot produce a tree that breaks later operations.
func (t *Tree) UnmarshalJSON(b []byte) error {
	type wire Tree
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidTree, err)
	}
	got := Tree(w)
	if err := got.validate(); err != nil {
		return err
	}
	*t = got
	return nil
}
