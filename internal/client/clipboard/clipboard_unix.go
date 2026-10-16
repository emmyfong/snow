//go:build !windows

package clipboard

import tea "charm.land/bubbletea/v2"

// Set returns a command that puts text on the system clipboard. It writes an
// OSC 52 sequence to the outer terminal, which has no way to say whether it
// accepted it, so the command never reports an error. Terminals that ignore
// OSC 52 (some disable it by default) ignore the copy.
func Set(text string) tea.Cmd { return tea.SetClipboard(text) }
