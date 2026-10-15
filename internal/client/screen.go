// Package client is Snow's terminal user interface. The root model routes
// between screens and owns the connection to the server.
package client

import tea "charm.land/bubbletea/v2"

// Screen is one full-screen view. Screens update themselves, so a screen
// package never imports this one.
type Screen interface {
	Update(msg tea.Msg) tea.Cmd
	View() tea.View
	SetSize(cols, rows int)
}
