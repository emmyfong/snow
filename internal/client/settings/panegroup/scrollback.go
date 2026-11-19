package panegroup

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
)

// Scrollback shows the configured scrollback and when it takes effect.
type Scrollback struct{ cfg config.Config }

// NewScrollback returns the scrollback settings group for cfg.
func NewScrollback(cfg config.Config) *Scrollback { return &Scrollback{cfg: cfg} }

// Title is the label shown in the settings group list.
func (*Scrollback) Title() string { return "Scrollback" }

// Update ignores input because scrollback settings are read-only.
func (*Scrollback) Update(tea.Msg) tea.Cmd { return nil }

// Leave has no work because scrollback settings are read-only.
func (*Scrollback) Leave() tea.Cmd { return nil }

// View draws the configured scrollback and its effective scope.
func (g *Scrollback) View(th theme.Theme, width, height int) string {
	return groupView(th, width, height, "Scrollback", fmt.Sprintf("%d lines", g.cfg.Scrollback.Lines), "each pane, after the server restarts", "[scrollback] lines")
}
