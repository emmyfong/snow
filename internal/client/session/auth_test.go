package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// authProviders is one provider with a saved key and one without.
func authProviders() []api.ProviderAuth {
	return []api.ProviderAuth{
		{Name: "anthropic", Title: "Anthropic", KeyURL: "https://console.anthropic.com/settings/keys",
			Source: "OS credential store", Connected: true},
		{Name: "openai", Title: "OpenAI"},
	}
}

func lastLoginRequest(r *recorder) *api.LoginRequest {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if req, ok := r.sent[i].(*api.LoginRequest); ok {
			return req
		}
	}
	return nil
}

func lastLogoutRequest(r *recorder) *api.LogoutRequest {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if req, ok := r.sent[i].(*api.LogoutRequest); ok {
			return req
		}
	}
	return nil
}

// loginOpen opens the login picker over a chat and feeds it a key list.
func loginOpen(t *testing.T, providers ...api.ProviderAuth) (*Model, *recorder) {
	t.Helper()
	m, r := chatScreen(t, 100, 30)
	typeSlash(m, "/login")
	if m.authPick == nil {
		t.Fatal("/login did not open the picker")
	}
	m.Update(&api.AuthList{Providers: providers})
	return m, r
}

// TestNewChatKnowsTheKeyState: a chat pane created after the key list
// arrived shows that a key exists, so it points at /model, not /login.
func TestNewChatKnowsTheKeyState(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Source: "environment", Connected: true}}})
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Rect: api.Rect{W: 30, H: 29}, Profile: "sh"},
		{ID: 2, Kind: api.PaneChat, Rect: api.Rect{X: 31, W: 30, H: 29}, Profile: "sh"},
		{ID: 3, Kind: api.PaneChat, Rect: api.Rect{X: 62, W: 38, H: 29}, Profile: "sh", Focused: true},
	}}}})
	m.Update(&api.ChatSnapshot{Pane: 3, Agent: "c2", Conversation: "c2", Mode: "ask", State: api.ChatStateIdle})
	s := ansi.Strip(content(m))
	if strings.Contains(s, "Connect a provider") {
		t.Fatalf("a chat with a key points at /login:\n%s", s)
	}
	if !strings.Contains(s, "Type /model to") {
		t.Fatalf("a chat with a key does not point at /model:\n%s", s)
	}
}

// TestLoginPicker: /login opens the picker and asks for the key state.
func TestLoginPicker(t *testing.T) {
	m, r := loginOpen(t, authProviders()...)
	if !sentAuthListRequest(r) {
		t.Fatal("/login did not ask for the key state")
	}
	s := ansi.Strip(content(m))
	for _, want := range []string{"Log in", "Anthropic", "OS credential store", "OpenAI", "no key"} {
		if !strings.Contains(s, want) {
			t.Errorf("the picker does not show %q:\n%s", want, s)
		}
	}
}

// TestLoginSubmitsTheKey: Enter on a provider opens the masked field; Enter
// sends the key, and a refusal returns to the field with the key and the
// reason.
func TestLoginSubmitsTheKey(t *testing.T) {
	m, r := loginOpen(t, authProviders()...)
	m.Update(press(tea.KeyEnter, 0, ""))
	for _, ch := range "sk-test-123" {
		m.Update(press(ch, 0, string(ch)))
	}
	s := ansi.Strip(content(m))
	if strings.Contains(s, "sk-test-123") || !strings.Contains(s, strings.Repeat("•", len("sk-test-123"))) {
		t.Fatalf("the key is not masked:\n%s", s)
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	req := lastLoginRequest(r)
	if req == nil || req.Provider != "anthropic" || req.Key != "sk-test-123" {
		t.Fatalf("sent %+v", req)
	}
	if !m.authPick.Checking() {
		t.Fatal("the picker does not say it is checking")
	}
	m.Update(&api.Error{Code: api.CodeAuthFailed, Message: "anthropic rejected the key"})
	if m.authPick == nil || m.authPick.Checking() {
		t.Fatal("a refusal closed the picker or left it checking")
	}
	if s := ansi.Strip(content(m)); !strings.Contains(s, "rejected the key") {
		t.Fatalf("the picker does not show the refusal:\n%s", s)
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	if req := lastLoginRequest(r); req == nil || req.Key != "sk-test-123" {
		t.Fatalf("the retry sent %+v", req)
	}
}

// TestLoginSaved: a saved key returns to the provider list with the source
// and the store's notice.
func TestLoginSaved(t *testing.T) {
	m, _ := loginOpen(t, authProviders()...)
	m.Update(&api.Login{Provider: "anthropic", LoginResult: api.LoginResult{
		Source: "OS credential store", Notice: "The fallback file is in use.",
	}})
	s := ansi.Strip(content(m))
	for _, want := range []string{"Saved to OS credential store.", "fallback file"} {
		if !strings.Contains(s, want) {
			t.Errorf("the picker does not show %q:\n%s", want, s)
		}
	}
}

// TestFirstChatWithoutKeyOpensLogin: a chat with no model and no key opens
// the login picker, and a saved key then opens the model picker.
func TestFirstChatWithoutKeyOpensLogin(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic"}}})
	if m.authPick == nil || m.modelPick != nil {
		t.Fatal("a chat with no key did not open the login picker")
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	for _, ch := range "sk-test-123" {
		m.Update(press(ch, 0, string(ch)))
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	if req := lastLoginRequest(r); req == nil || req.Provider != "anthropic" {
		t.Fatalf("sent %+v", req)
	}
	m.Update(&api.Login{Provider: "anthropic", LoginResult: api.LoginResult{Source: "OS credential store"}})
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Connected: true, Source: "OS credential store"}}})
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.authPick != nil {
		t.Fatal("Esc left the login picker open")
	}
	if m.modelPick == nil || !sentListRequest(r) {
		t.Fatal("a saved key did not open the model picker")
	}
}

// TestLoginCancelDuringCheck: closing the picker while the server checks
// still reports the saved key and opens the model picker for an empty chat.
func TestLoginCancelDuringCheck(t *testing.T) {
	m, _ := modelScreen(t, 100, 30, "")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic"}}})
	m.Update(press(tea.KeyEnter, 0, ""))
	for _, ch := range "sk-test-123" {
		m.Update(press(ch, 0, string(ch)))
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.authPick != nil {
		t.Fatal("Esc left the login picker open")
	}
	m.Update(&api.Login{Provider: "anthropic", LoginResult: api.LoginResult{Source: "OS credential store"}})
	if !strings.Contains(m.notice, "Saved to OS credential store.") {
		t.Fatalf("notice = %q", m.notice)
	}
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Connected: true, Source: "OS credential store"}}})
	if m.modelPick == nil {
		t.Fatal("a saved key did not open the model picker")
	}
}

// TestLogoutConfirm: n and Esc keep the key; y sends the logout request.
func TestLogoutConfirm(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  tea.KeyPressMsg
		want bool
	}{
		{"no", press('n', 0, "n"), false},
		{"escape", press(tea.KeyEscape, 0, ""), false},
		{"yes", press('y', 0, "y"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, r := chatScreen(t, 100, 30)
			typeSlash(m, "/logout")
			if !sentAuthListRequest(r) {
				t.Fatal("/logout did not ask for the key state")
			}
			m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic",
				Source: "OS credential store", Connected: true}}})
			if m.confirm == nil {
				t.Fatal("/logout did not ask for confirmation")
			}
			m.Update(tt.key)
			if m.confirm != nil {
				t.Fatal("the confirmation stayed open")
			}
			req := lastLogoutRequest(r)
			if tt.want && (req == nil || req.Provider != "anthropic") {
				t.Fatalf("sent %+v, want a logout for anthropic", req)
			}
			if !tt.want && req != nil {
				t.Fatalf("sent %+v", req)
			}
		})
	}
}

// TestLogoutEnvKey: a key from the environment cannot be removed, and the
// notice says why.
func TestLogoutEnvKey(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	typeSlash(m, "/logout")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic",
		Source: "environment", EnvVar: "ANTHROPIC_API_KEY", Connected: true}}})
	if m.confirm != nil {
		t.Fatal("an environment key asked for confirmation")
	}
	if req := lastLogoutRequest(r); req != nil {
		t.Fatalf("sent %+v", req)
	}
	if !strings.Contains(m.message, "ANTHROPIC_API_KEY") || !strings.Contains(m.message, "cannot remove") {
		t.Fatalf("status = %q", m.message)
	}
}

// TestLogoutUnreadableKey: when the chat's provider has a saved key that
// cannot be read, /logout says why and leaves other keys alone.
func TestLogoutUnreadableKey(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "openai/gpt-6.1-sol", Mode: "ask", State: api.ChatStateIdle})
	typeSlash(m, "/logout")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{
		{Name: "anthropic", Title: "Anthropic", Source: "OS credential store", Connected: true},
		{Name: "openai", Title: "OpenAI", Err: "the key store did not answer"},
	}})
	if m.confirm != nil {
		t.Fatal("an unreadable key asked for confirmation")
	}
	if req := lastLogoutRequest(r); req != nil {
		t.Fatalf("sent %+v", req)
	}
	if !strings.Contains(m.message, "Cannot read the key for OpenAI") {
		t.Fatalf("status = %q", m.message)
	}
}

// TestLogoutUsesChatProvider: with two keys, /logout removes the chat's
// provider.
func TestLogoutUsesChatProvider(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "openai/gpt-6.1-sol", Mode: "ask", State: api.ChatStateIdle})
	typeSlash(m, "/logout")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{
		{Name: "anthropic", Title: "Anthropic", Source: "OS credential store", Connected: true},
		{Name: "openai", Title: "OpenAI", Source: "OS credential store", Connected: true},
	}})
	if m.confirm == nil {
		t.Fatal("/logout did not ask for confirmation")
	}
	m.Update(press('y', 0, "y"))
	if req := lastLogoutRequest(r); req == nil || req.Provider != "openai" {
		t.Fatalf("sent %+v, want a logout for openai", req)
	}
}

// TestLogoutNoKey: with no saved key, /logout says how to save one.
func TestLogoutNoKey(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	typeSlash(m, "/logout")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic"}}})
	if m.confirm != nil {
		t.Fatal("a missing key asked for confirmation")
	}
	if req := lastLogoutRequest(r); req != nil {
		t.Fatalf("sent %+v", req)
	}
	if !strings.Contains(m.message, "No provider has a saved key") {
		t.Fatalf("status = %q", m.message)
	}
}

// TestLoginOverlayGolden draws the login picker over a chat.
func TestLoginOverlayGolden(t *testing.T) {
	m, _ := loginOpen(t, authProviders()...)
	golden.RequireEqual(t, content(m))
}

// TestLogoutSkipsNoKeyProvider: /logout never offers to remove the key of
// a provider that takes none, even when the chat uses it.
func TestLogoutSkipsNoKeyProvider(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "omniroute/cc/claude-x", Mode: "ask", State: api.ChatStateIdle})
	typeSlash(m, "/logout")
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{
		{Name: "omniroute", Connected: true, NoKey: true},
		{Name: "openai", Title: "OpenAI", Source: "OS credential store", Connected: true},
	}})
	if m.confirm == nil {
		t.Fatal("/logout did not ask about the saved key")
	}
	m.Update(press('y', 0, "y"))
	if req := lastLogoutRequest(r); req == nil || req.Provider != "openai" {
		t.Fatalf("sent %+v, want a logout for openai", req)
	}
}
