package session

import (
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
)

// Option changes how New builds the screen.
type Option func(*Model)

// WithTheme sets the theme the screen draws with.
func WithTheme(th theme.Theme) Option { return func(m *Model) { m.theme = th } }

// WithKeys sets the key binding table. The default is tmux's keys with the
// prefix Ctrl+b.
func WithKeys(t *keys.Table) Option { return func(m *Model) { m.keys = t } }
