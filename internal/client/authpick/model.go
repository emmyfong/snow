// Package authpick is the login picker: the popup that lists providers and
// the state of their API keys, asks for a key in a masked field, and shows
// what the server's check did.
package authpick

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// stage is what the picker asks for: a provider, a key, or an answer from
// the server.
type stage int

// Stages.
const (
	stageList stage = iota
	stageKey
	stageChecking
)

// Result is what a key press or click did to the picker.
type Result int

// Results.
const (
	Open      Result = iota // the picker stays open
	Cancelled               // the user closed the picker
	Submit                  // send a LoginRequest for Provider and KeyValue
)

// maxKey bounds the key field so a stray paste cannot grow without bound.
const maxKey = 512

// Model is one open login picker.
type Model struct {
	providers []api.ProviderAuth
	selected  int
	ready     bool // SetList was called

	stage stage
	key   []rune
	note  string // a saved note, a refusal, or why the key cannot be sent
}

// New opens a login picker on its provider list.
func New() *Model { return &Model{} }

// SetList replaces the providers and their key states. The picker keeps the
// provider it had selected.
func (m *Model) SetList(providers []api.ProviderAuth) {
	name := m.providerName()
	m.providers, m.ready = providers, true
	m.selected = 0
	for i, p := range providers {
		if p.Name == name {
			m.selected = i
			break
		}
	}
	if m.stage != stageList && len(providers) == 0 {
		m.stage, m.key = stageList, nil
	}
}

// providerName returns the selected provider's name, or "".
func (m *Model) providerName() string {
	if m.selected < 0 || m.selected >= len(m.providers) {
		return ""
	}
	return m.providers[m.selected].Name
}

// Provider returns the selected provider.
func (m *Model) Provider() (api.ProviderAuth, bool) {
	if m.selected < 0 || m.selected >= len(m.providers) {
		return api.ProviderAuth{}, false
	}
	return m.providers[m.selected], true
}

// KeyValue returns the key as typed, without the spaces or line ending a
// paste can carry.
func (m *Model) KeyValue() string { return strings.TrimSpace(string(m.key)) }

// Checking reports whether a login request is waiting for the server.
func (m *Model) Checking() bool { return m.stage == stageChecking }

// Key handles one key press.
func (m *Model) Key(k tea.Key) Result {
	switch m.stage {
	case stageList:
		return m.listKey(k)
	case stageKey:
		return m.keyKey(k)
	case stageChecking:
		// The request is already on its way; closing leaves it to finish.
		if k.Code == tea.KeyEscape {
			return Cancelled
		}
	}
	return Open
}

// listKey handles a key on the provider list.
func (m *Model) listKey(k tea.Key) Result {
	switch {
	case k.Code == tea.KeyEscape:
		return Cancelled
	case k.Code == tea.KeyUp || k.Text == "k":
		m.selected = move(m.selected, -1, len(m.providers))
	case k.Code == tea.KeyDown || k.Text == "j":
		m.selected = move(m.selected, 1, len(m.providers))
	case k.Code == tea.KeyEnter:
		m.startKey()
	}
	return Open
}

// startKey opens the key field for the selected provider.
func (m *Model) startKey() {
	if len(m.providers) == 0 {
		m.note = "No providers."
		return
	}
	m.stage, m.key, m.note = stageKey, nil, ""
}

// keyKey handles a key while the key field has the keyboard.
func (m *Model) keyKey(k tea.Key) Result {
	switch {
	case k.Code == tea.KeyEscape:
		m.stage, m.key, m.note = stageList, nil, ""
	case k.Code == tea.KeyEnter:
		return m.submit()
	case k.Code == tea.KeyBackspace:
		if len(m.key) > 0 {
			m.key = m.key[:len(m.key)-1]
		}
		m.note = ""
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		m.key, m.note = nil, ""
	default:
		if text, ok := keys.Typed(k); ok {
			m.key = append(m.key, []rune(text)...)
			if len(m.key) > maxKey {
				m.key = m.key[:maxKey]
			}
			m.note = ""
		}
	}
	return Open
}

// submit sends a key the provider can check, or says why it cannot.
func (m *Model) submit() Result {
	key := m.KeyValue()
	switch {
	case key == "":
		m.note = "Type the API key."
		return Open
	case strings.ContainsAny(key, " \t\r\n"):
		m.note = "An API key is one word with no spaces."
		return Open
	}
	m.stage, m.note = stageChecking, ""
	return Submit
}

// Paste adds pasted text to the key field.
func (m *Model) Paste(text string) {
	if m.stage != stageKey {
		return
	}
	m.key = append(m.key, []rune(text)...)
	if len(m.key) > maxKey {
		m.key = m.key[:maxKey]
	}
	m.note = ""
}

// Fail returns to the key field after the server refused the key or could
// not check it. The key stays, so the user can correct it and retry.
func (m *Model) Fail(msg string) {
	if m.stage != stageChecking {
		return
	}
	m.stage, m.note = stageKey, msg
}

// Saved returns to the provider list after the server saved the key, with a
// note that says where it went.
func (m *Model) Saved(l *api.Login) {
	if l == nil {
		return
	}
	m.stage, m.key, m.note = stageList, nil, SavedText(l)
	for i, p := range m.providers {
		if p.Name == l.Provider {
			m.selected = i
			break
		}
	}
}

// SavedText says what a successful login did: where the key went, whether
// an environment variable still holds a key, and the credential store's
// one-time notice.
func SavedText(l *api.Login) string {
	var b strings.Builder
	if l.Source != "" {
		b.WriteString("Saved to " + l.Source + ".")
	} else {
		b.WriteString("Saved.")
	}
	if l.Shadowed {
		b.WriteString(" An environment variable holds a key for this provider, and Snow uses that one first.")
	}
	if l.Notice != "" {
		b.WriteString("\n" + l.Notice)
	}
	return b.String()
}

// move returns i moved by delta in a list of n items, wrapping at either
// end. It returns 0 for an empty list.
func move(i, delta, n int) int {
	if n <= 0 {
		return 0
	}
	return ((i+delta)%n + n) % n
}
