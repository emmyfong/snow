package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestAuthMessageNumbers pins the numbers of the auth messages: a released
// type is never renumbered, or old clients read the wrong message.
func TestAuthMessageNumbers(t *testing.T) {
	for _, tt := range []struct {
		got  MsgType
		want MsgType
	}{
		{TypeAuthListRequest, 27},
		{TypeAuthList, 28},
		{TypeLoginRequest, 29},
		{TypeLogin, 30},
		{TypeLogoutRequest, 31},
		{TypeLogout, 32},
	} {
		if tt.got != tt.want {
			t.Errorf("type is %d, want %d", tt.got, tt.want)
		}
	}
}

// TestAuthMessageRoundTrip: the auth messages travel by their pinned types.
func TestAuthMessageRoundTrip(t *testing.T) {
	for _, m := range []Message{
		&AuthListRequest{},
		&AuthList{Providers: []ProviderAuth{{Name: "anthropic", Title: "Anthropic (Claude)", KeyURL: "https://example.test/keys",
			Source: "environment", EnvVar: "ANTHROPIC_API_KEY", Connected: true}}},
		&LoginRequest{Provider: "anthropic", Key: "sk-test"},
		&Login{Provider: "anthropic", LoginResult: LoginResult{Source: "OS credential store", Shadowed: true, Notice: "using the fallback file"}},
		&LogoutRequest{Provider: "anthropic"},
		&Logout{Provider: "anthropic"},
	} {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		got := newMessage(m.Type())
		if got == nil {
			t.Fatalf("%T: no message for type %d", m, m.Type())
		}
		if err := json.Unmarshal(raw, got); err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		if a, b := string(raw), mustJSON(t, got); a != b {
			t.Errorf("%T round trip:\n%s\n%s", m, a, b)
		}
	}
}

// TestAuthListJSON: an empty provider list travels as [], never null.
func TestAuthListJSON(t *testing.T) {
	b, err := json.Marshal(AuthList{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); !strings.Contains(got, `"providers":[]`) {
		t.Fatalf("AuthList JSON = %s", got)
	}
}

// TestLoginRequestHidesKey: no fmt verb prints the key, but JSON carries it
// once for the server.
func TestLoginRequestHidesKey(t *testing.T) {
	const secret = "sk-SECRET"
	r := LoginRequest{Provider: "anthropic", Key: secret}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
		if got := fmt.Sprintf(format, r); strings.Contains(got, secret) {
			t.Errorf("%s of LoginRequest = %q, leaks the key", format, got)
		}
	}
	if got := fmt.Sprintf("%v", &r); strings.Contains(got, secret) {
		t.Errorf("%%v of *LoginRequest = %q, leaks the key", got)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); !strings.Contains(got, `"key":"`+secret+`"`) {
		t.Fatalf("LoginRequest JSON = %s, want the key", got)
	}
}

// TestAuthValidate: a request without a provider or a key is refused.
func TestAuthValidate(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		ok   bool
	}{
		{"login full", (&LoginRequest{Provider: "anthropic", Key: "sk-test"}).Validate(), true},
		{"login no provider", (&LoginRequest{Key: "sk-test"}).Validate(), false},
		{"login no key", (&LoginRequest{Provider: "anthropic"}).Validate(), false},
		{"logout full", (&LogoutRequest{Provider: "anthropic"}).Validate(), true},
		{"logout no provider", (&LogoutRequest{}).Validate(), false},
	} {
		if got := tt.err == nil; got != tt.ok {
			t.Errorf("%s: error %v, want ok %v", tt.name, tt.err, tt.ok)
		}
		if tt.err != nil && !errors.Is(tt.err, ErrInvalidAuth) {
			t.Errorf("%s: error %v does not match ErrInvalidAuth", tt.name, tt.err)
		}
	}
}
