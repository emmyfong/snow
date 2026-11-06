package credential

import (
	"errors"
	"strings"
	"testing"
)

func TestStoredKeyIgnoresEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name string
		os   bool
	}{
		{name: "OS store", os: true},
		{name: "private file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := newFakeOS()
			o.broken = !tc.os
			s := newStore(env(map[string]string{"ANTHROPIC_API_KEY": "machine-key"}), o, t.TempDir(), nil)
			if _, _, err := s.GetStored("anthropic"); !errors.Is(err, ErrNoKey) || strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
				// The environment never supplies a stored key, so the
				// error must not tell the user to set it there.
				t.Fatalf("unsaved connection: %v, want ErrNoKey with no variable hint", err)
			}
			if _, err := s.Set("anthropic", mustKey(t, "snow-key")); err != nil {
				t.Fatal(err)
			}
			key, source, err := s.GetStored("anthropic")
			if err != nil || key.Reveal() != "snow-key" || source == SourceEnv {
				t.Fatalf("stored lookup used the wrong credential source: %v, %v", source, err)
			}
			key, source, err = s.Get("anthropic")
			if err != nil || key.Reveal() != "machine-key" || source != SourceEnv {
				t.Fatalf("legacy lookup changed: %v, %v", source, err)
			}
			if err := s.Delete("anthropic"); !errors.Is(err, ErrInEnvironment) {
				t.Fatalf("legacy delete changed: %v", err)
			}
			if err := s.DeleteStored("anthropic"); err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.GetStored("anthropic"); !errors.Is(err, ErrNoKey) {
				t.Fatalf("saved key remains after deletion: %v", err)
			}
			key, source, err = s.Get("anthropic")
			if err != nil || key.Reveal() != "machine-key" || source != SourceEnv {
				t.Fatalf("removing saved key changed the host lookup: %v, %v", source, err)
			}
		})
	}
}
