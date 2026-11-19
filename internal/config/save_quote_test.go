package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestQuote(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty"},
		{name: "quotes", value: `say "hello"`},
		{name: "backslashes", value: `C:\snow\config.toml`},
		{name: "tab", value: "before\tafter"},
		{name: "newline", value: "before\nafter"},
		{name: "control byte", value: "\x01"},
		{name: "bell", value: "\a"},
		{name: "vertical tab", value: "\v"},
		{name: "delete", value: "\x7f"},
		{name: "non-ASCII", value: "Snow 雪"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quoted := Quote(tt.value)
			for _, forbidden := range []string{`\x`, `\a`, `\v`} {
				if strings.Contains(quoted, forbidden) {
					t.Fatalf("Quote(%q) = %q, contains TOML-invalid escape %q", tt.value, quoted, forbidden)
				}
			}
			var decoded map[string]string
			if _, err := toml.Decode("value = "+quoted, &decoded); err != nil {
				t.Fatalf("decode Quote(%q) %q: %v", tt.value, quoted, err)
			}
			if got := decoded["value"]; got != tt.value {
				t.Fatalf("Quote(%q) round-tripped as %q", tt.value, got)
			}
		})
	}
}
