package clauderuntime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writePolicy(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestPolicyCheck: each managed rule refuses exactly the routes it rules
// out, and a drop-in file replaces the main file's value, as Claude merges.
func TestPolicyCheck(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		sub, key bool // allowed
	}{
		{"no policy", nil, true, true},
		{"claude.ai only", map[string]string{"managed-settings.json": `{"forceLoginMethod":"claudeai"}`}, true, false},
		{"console only", map[string]string{"managed-settings.json": `{"forceLoginMethod":"console"}`}, false, false},
		{"gateway", map[string]string{"managed-settings.json": `{"forceLoginMethod":"gateway"}`}, false, false},
		{"key helper", map[string]string{"managed-settings.json": `{"apiKeyHelper":"/bin/key"}`}, false, true},
		{"env key", map[string]string{"managed-settings.json": `{"env":{"ANTHROPIC_API_KEY":"x"}}`}, false, true},
		{"drop-in wins", map[string]string{"managed-settings.json": `{"forceLoginMethod":"console"}`,
			"managed-settings.d/10-a.json": `{"forceLoginMethod":"claudeai"}`}, true, false},
		{"hidden drop-in ignored", map[string]string{"managed-settings.d/.x.json": `{"forceLoginMethod":"console"}`}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tt.files {
				writePolicy(t, dir, name, body)
			}
			p, err := ReadPolicy(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range []struct {
				auth AuthMode
				ok   bool
			}{{Subscription, tt.sub}, {APIKey, tt.key}} {
				err := p.Check(c.auth)
				if c.ok != (err == nil) || (err != nil && !errors.Is(err, ErrPolicy)) {
					t.Errorf("%s: err = %v, want allowed %v", c.auth, err, c.ok)
				}
			}
		})
	}
}

// TestPolicyUnreadableIsAnError: a policy file Snow cannot parse is not
// treated as no policy.
func TestPolicyUnreadableIsAnError(t *testing.T) {
	dir := t.TempDir()
	writePolicy(t, dir, "managed-settings.json", `{not json`)
	if _, err := ReadPolicy(dir); err == nil {
		t.Fatal("broken policy read as no policy")
	}
}
