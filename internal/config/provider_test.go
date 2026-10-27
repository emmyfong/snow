package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestConfigProviders(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int    // of the error; 0 means the file loads
		key  string // of the error
	}{
		{"valid", `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
no_key = true

[provider.open-router]
base_url = "https://openrouter.ai/api/v1"
models = ["qwen/qwen3-coder:free"]
`, 0, ""},
		{"missing URL", `
[provider.omniroute]
no_key = true
`, 2, "provider.omniroute.base_url"},
		{"bad scheme", `
[provider.omniroute]
base_url = "ftp://localhost/v1"
`, 3, "provider.omniroute.base_url"},
		{"no host", `
[provider.omniroute]
base_url = "http:///v1"
`, 3, "provider.omniroute.base_url"},
		{"key in the URL", `
[provider.omniroute]
base_url = "https://user:sk-secret@example.com/v1"
`, 3, "provider.omniroute.base_url"},
		{"query", `
[provider.omniroute]
base_url = "https://example.com/v1?key=x"
`, 3, "provider.omniroute.base_url"},
		{"upper-case name", `
[chat]
usage = true

[provider.OmniRoute]
base_url = "http://localhost:20128/v1"
`, 5, "provider.OmniRoute"},
		{"slash in name", `
[provider."omni/route"]
base_url = "http://localhost:20128/v1"
`, 2, `provider."omni/route"`},
		{"blank model", `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
models = ["a", " "]
`, 4, "provider.omniroute.models"},
		{"unknown key", `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
api_key = "sk-secret"
`, 4, "provider.omniroute.api_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.body))
			if tt.line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]Endpoint{
					"omniroute":   {BaseURL: "http://localhost:20128/v1", NoKey: true},
					"open-router": {BaseURL: "https://openrouter.ai/api/v1", Models: []string{"qwen/qwen3-coder:free"}},
				}
				if !reflect.DeepEqual(cfg.Providers, want) {
					t.Fatalf("providers %+v, want %+v", cfg.Providers, want)
				}
				return
			}
			var ce *Error
			if !errors.As(err, &ce) || ce.Line != tt.line || ce.Key != tt.key {
				t.Fatalf("error %v (%+v), want line %d key %s", err, ce, tt.line, tt.key)
			}
			if strings.Contains(err.Error(), "sk-secret") {
				t.Fatalf("the error shows the key: %v", err)
			}
		})
	}
}
