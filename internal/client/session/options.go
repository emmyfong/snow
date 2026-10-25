package session

import (
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
)

// Option changes how New builds the screen.
type Option func(*Model)

// WithTheme sets the theme the screen draws with.
func WithTheme(th theme.Theme) Option { return func(m *Model) { m.theme, m.base = th, th } }

// WithKeys sets the key binding table. The default is tmux's keys with the
// prefix Ctrl+b.
func WithKeys(t *keys.Table) Option { return func(m *Model) { m.keys = t } }

// WithChatUsage sets whether chat panes show the context gauge and the
// cost, from the [chat] usage setting. The default shows them.
func WithChatUsage(show bool) Option { return func(m *Model) { m.hideUsage = !show } }

// SetTheme changes the theme of an open screen, for example when another
// Snow window picked a new one. It is adapted to the terminal's colors once
// they are known.
func (m *Model) SetTheme(th theme.Theme) {
	m.base, m.theme = th, th
	if m.profile != nil {
		m.theme = th.For(*m.profile)
	}
}
