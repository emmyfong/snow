package theme

import (
	"errors"
	"testing"
)

func TestNamed(t *testing.T) {
	if _, err := Named("default"); err != nil {
		t.Fatalf("Named(default) error = %v", err)
	}
	_, err := Named("neon")
	if !errors.Is(err, ErrUnknownTheme) {
		t.Fatalf("Named(neon) error = %v, want ErrUnknownTheme", err)
	}
}
