package session

import "github.com/emmyfong/snow/internal/client/theme"

// Option changes how New builds the screen.
type Option func(*Model)

// WithTheme sets the theme the screen draws with.
func WithTheme(th theme.Theme) Option { return func(m *Model) { m.theme = th } }
