package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// named is a tool that only has a name.
type named string

func (n named) Def() Def { return Def{Name: string(n)} }

func (named) Run(context.Context, Env, json.RawMessage) (string, error) { return "", nil }

func TestRegistryDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(named("read")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(named("read")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second register: %v, want ErrDuplicate", err)
	}
}

func TestRegistryDefsSorted(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"write", "edit", "read"} {
		if err := r.Register(named(n)); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, d := range r.Defs() {
		got = append(got, d.Name)
	}
	if want := "edit read write"; strings.Join(got, " ") != want {
		t.Fatalf("Defs = %v, want %s", got, want)
	}
}

// TestBuiltinIsACopy: a tool added to Builtin's result does not join the
// built-in set.
func TestBuiltinIsACopy(t *testing.T) {
	if err := Builtin().Register(named("extra")); err != nil {
		t.Fatal(err)
	}
	if _, ok := Builtin().Get("extra"); ok {
		t.Fatal("a tool added to a copy joined the built-in set")
	}
}
