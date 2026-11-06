package clauderuntime

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAuthStatusRejectsRouteAndProfileConflicts(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mode   AuthMode
		method string
		in     bool
		code   int
		dir    string
		want   error
	}{
		{"subscription", Subscription, "claude.ai", true, 0, p.ConfigDir, nil},
		{"API key", APIKey, "api_key", true, 0, p.ConfigDir, nil},
		{"signed out", Subscription, "none", false, 1, p.ConfigDir, nil},
		{"host profile", Subscription, "claude.ai", true, 0, "standalone", ErrProtocol},
		{"missing profile evidence", Subscription, "claude.ai", true, 0, "", ErrProtocol},
		{"subscription became API billing", Subscription, "api_key", true, 0, p.ConfigDir, ErrCredential},
		{"API key became subscription", APIKey, "claude.ai", true, 0, p.ConfigDir, ErrCredential},
		{"inherited helper", APIKey, "api_key_helper", true, 0, p.ConfigDir, ErrCredential},
		{"inherited token", Subscription, "oauth_token", true, 0, p.ConfigDir, ErrCredential},
		{"crash is not signed out", Subscription, "none", false, 2, p.ConfigDir, ErrProtocol},
		{"contradictory exit", Subscription, "none", false, 0, p.ConfigDir, ErrProtocol},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p.Auth = tc.mode
			data, err := json.Marshal(map[string]any{"loggedIn": tc.in, "authMethod": tc.method, "configDirectory": tc.dir})
			if err != nil {
				t.Fatal(err)
			}
			status, err := parseAuthStatus(data, tc.code, p)
			if !errors.Is(err, tc.want) || tc.want == nil && (status.SignedIn != tc.in || status.Method != tc.method) {
				t.Fatalf("authentication evidence: %+v, %v", status, err)
			}
		})
	}
	if _, err := parseAuthStatus([]byte(`{}`), 0, p); !errors.Is(err, ErrProtocol) {
		t.Fatalf("unknown schema became signed out: %v", err)
	}
}
