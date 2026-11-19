package session

import (
	"image/color"

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
	if m.prompt != nil && m.prompt.mode == modeThemes && m.prompt.themePick != nil {
		m.base = th
		m.prompt.themePick.SetBefore(th)
		return
	}
	m.base = th
	m.previewTheme(th)
}

// SetPreviewTheme changes the visible theme without changing the restore point.
func (m *Model) SetPreviewTheme(th theme.Theme) { m.previewTheme(th) }

// CurrentTheme returns the raw theme shown by the session or its theme picker.
func (m *Model) CurrentTheme() theme.Theme {
	if m.prompt != nil && m.prompt.mode == modeThemes && m.prompt.themePick != nil {
		return m.prompt.themePick.Shown()
	}
	return m.base
}

func (m *Model) applyTheme(th theme.Theme) {
	m.theme = th
	for _, p := range m.own {
		p.SetTheme(m.theme)
	}
}

// SetPaintedBackground adapts secondary text to the background Snow applies.
// A nil color means the terminal supplies its own background.
func (m *Model) SetPaintedBackground(bg color.Color) {
	if m.paintedBackground == bg {
		return
	}
	m.paintedBackground = bg
	if m.prompt != nil && m.prompt.mode == modeThemes && m.prompt.themePick != nil {
		m.previewTheme(m.prompt.themePick.Shown())
		return
	}
	m.previewTheme(m.base)
}
