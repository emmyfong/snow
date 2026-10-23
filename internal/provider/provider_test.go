package provider_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// TestErrorKinds: every kind matches with errors.As, keeps its cause, and
// says what went wrong.
func TestErrorKinds(t *testing.T) {
	cause := errors.New("status 400")
	for _, k := range []provider.Kind{
		provider.KindBadKey, provider.KindNoKey, provider.KindModelNotFound, provider.KindRateLimited,
		provider.KindOverloaded, provider.KindContextTooLong, provider.KindNetwork, provider.KindOther,
	} {
		var err error = &provider.Error{Kind: k, Err: cause}
		err = errors.Join(errors.New("turn failed"), err)
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != k {
			t.Errorf("kind %v: errors.As = %v", k, pe)
			continue
		}
		if !errors.Is(err, cause) {
			t.Errorf("kind %v lost its cause", k)
		}
		if !strings.Contains(pe.Error(), k.String()) || !strings.Contains(pe.Error(), "status 400") {
			t.Errorf("kind %v: message %q", k, pe.Error())
		}
	}
}

// TestRateLimitedRetryAfter: the retry time is on the error and in its text.
func TestRateLimitedRetryAfter(t *testing.T) {
	err := error(&provider.Error{Kind: provider.KindRateLimited, RetryAfter: 30 * time.Second})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v", pe)
	}
	if !strings.Contains(err.Error(), "retry in 30s") {
		t.Fatalf("message %q lacks the retry time", err.Error())
	}
}

func TestParseModelName(t *testing.T) {
	tests := []struct {
		in   string
		want provider.ModelName
		ok   bool
	}{
		{"anthropic/claude-opus-5-5", provider.ModelName{Provider: "anthropic", Model: "claude-opus-5-5"}, true},
		{"router/anthropic/claude", provider.ModelName{Provider: "router", Model: "anthropic/claude"}, true},
		{"", provider.ModelName{}, false},
		{"claude-opus-5-5", provider.ModelName{}, false},
		{"/claude", provider.ModelName{}, false},
		{"anthropic/", provider.ModelName{}, false},
	}
	for _, tt := range tests {
		got, err := provider.ParseModelName(tt.in)
		if tt.ok != (err == nil) || got != tt.want {
			t.Errorf("ParseModelName(%q) = %v, %v", tt.in, got, err)
		}
		if !tt.ok && !errors.Is(err, provider.ErrBadModelName) {
			t.Errorf("ParseModelName(%q) error %v is not ErrBadModelName", tt.in, err)
		}
		if tt.ok && got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestRegistry(t *testing.T) {
	r := provider.NewRegistry()
	a, b := providertest.New("a"), providertest.New("b")
	for _, p := range []provider.Provider{b, a} {
		if err := r.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	if p, ok := r.Get("a"); !ok || p != a {
		t.Fatalf("Get(a) = %v, %v", p, ok)
	}
	if p, ok := r.Get("b"); !ok || p != b {
		t.Fatalf("Get(b) = %v, %v", p, ok)
	}
	if _, ok := r.Get("c"); ok {
		t.Fatal("Get(c) found a provider")
	}
	if err := r.Register(providertest.New("a")); !errors.Is(err, provider.ErrDuplicate) {
		t.Fatalf("duplicate Register = %v", err)
	}
	if got := r.Names(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Names = %v", got)
	}
}

func TestUndated(t *testing.T) {
	tests := map[string]string{
		"claude-opus-5-5-20261001": "claude-opus-5-5",
		"claude-opus-5-5":          "claude-opus-5-5",
		"claude-opus-5-5-2026100x": "claude-opus-5-5-2026100x",
		"claude-5-2026":            "claude-5-2026",
		"-20261001":                "-20261001",
		"":                         "",
	}
	for in, want := range tests {
		if got := provider.Undated(in); got != want {
			t.Errorf("Undated(%q) = %q, want %q", in, got, want)
		}
	}
}
