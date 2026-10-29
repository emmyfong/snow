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
		{"long name", `
[provider.a-very-long-provider-name-of-33-chr]
base_url = "http://localhost:20128/v1"
`, 2, "provider.a-very-long-provider-name-of-33-chr"},
		{"leading dash", `
[provider.-gw]
base_url = "http://localhost:20128/v1"
`, 2, "provider.-gw"},
		{"empty name", `
[provider.""]
base_url = "http://localhost:20128/v1"
`, 2, `provider.""`},
		{"model with a space", `
[provider.omniroute]
base_url = "http://localhost:20128/v1"
models = ["auto best"]
`, 4, "provider.omniroute.models"},
		{"first bad table by name", `
[provider.zeta]
base_url = "ftp://z"

[provider.alpha]
base_url = "ftp://a"
`, 6, "provider.alpha.base_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.body))
			if tt.line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]Endpoint{
					"omniroute":   {BaseURL: "http://localhost:20128/v1", NoKey: true, API: APIResponses},
					"open-router": {BaseURL: "https://openrouter.ai/api/v1", Models: []string{"qwen/qwen3-coder:free"}, API: APIResponses},
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

// TestConfigProvidersCRLF: a file with Windows line endings reports the
// same lines.
func TestConfigProvidersCRLF(t *testing.T) {
	for _, tt := range []struct {
		body string
		line int
	}{
		{"\n[provider.omniroute]\nno_key = true\n", 2},
		{"\n[provider.omniroute]\nbase_url = \"http://localhost:20128/v1\"\napi_key = \"x\"\n", 4},
		{"\n[provider.omniroute]\nbase_url = \"http://localhost:20128/v1\"\napi = \"completions\"\n", 4},
	} {
		_, err := Load(writeConfig(t, strings.ReplaceAll(tt.body, "\n", "\r\n")))
		var ce *Error
		if !errors.As(err, &ce) || ce.Line != tt.line {
			t.Fatalf("error %v, want line %d", err, tt.line)
		}
	}
}

// TestConfigProviderAPI: api picks the API a service speaks. Without it,
// the service speaks the Responses API; a value that is neither names its
// line (#225).
func TestConfigProviderAPI(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string // the API, when the file loads
		line int    // of the error, when it does not
	}{
		{"responses", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\napi = \"responses\"\n", APIResponses, 0},
		{"chat", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\napi = \"chat\"\n", APIChat, 0},
		{"no value", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\n", APIResponses, 0},
		{"bad value", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\napi = \"completions\"\n", "", 4},
		{"wrong case", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\napi = \"Chat\"\n", "", 4},
		{"empty value is the default", "\n[provider.gw]\nbase_url = \"http://localhost:1/v1\"\napi = \"\"\n", APIResponses, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.body))
			if tt.line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if got := cfg.Providers["gw"].API; got != tt.want {
					t.Fatalf("API %q, want %q", got, tt.want)
				}
				return
			}
			var ce *Error
			if !errors.As(err, &ce) || ce.Line != tt.line || ce.Key != "provider.gw.api" {
				t.Fatalf("error %v (%+v), want line %d key provider.gw.api", err, ce, tt.line)
			}
		})
	}
}
