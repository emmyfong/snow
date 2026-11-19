// Package panegroup holds the read-only profile and scrollback settings groups.
package panegroup

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
)

// Profiles shows the configured default profile and when it takes effect.
type Profiles struct{ cfg config.Config }

// NewProfiles returns the profile settings group for cfg.
func NewProfiles(cfg config.Config) *Profiles { return &Profiles{cfg: cfg} }

// Title is the label shown in the settings group list.
func (*Profiles) Title() string { return "Profiles" }

// Update ignores input because profile settings are read-only.
func (*Profiles) Update(tea.Msg) tea.Cmd { return nil }

// Leave has no work because profile settings are read-only.
func (*Profiles) Leave() tea.Cmd { return nil }

// View draws the configured profile and its effective scope.
func (g *Profiles) View(th theme.Theme, width, height int) string {
	value := g.cfg.Profile.Default
	if value == "" {
		value = "first detected"
	}
	return groupView(th, width, height, "Default profile", value, "new panes, after the server restarts", "[profile] default")
}
