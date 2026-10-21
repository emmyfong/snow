package session

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// renaming is a window name being typed in the status bar, as tmux does for
// its rename key.
type renaming struct {
	window int
	text   []rune
}

// maxWindowName bounds a typed name; longer names would only be cut by the
// status bar.
const maxWindowName = 32

// startRename asks for a new name for the shown window, starting from the
// current one.
func (m *Model) startRename() {
	w := m.shown()
	if w == nil {
		return
	}
	m.rename = &renaming{window: w.Index, text: []rune(w.Name)}
}

// renameKey edits the name: Enter keeps it, Escape cancels, and an empty
// name is not sent, because the server refuses one.
func (m *Model) renameKey(k tea.Key) {
	r := m.rename
	switch k.Code {
	case tea.KeyEscape:
		m.rename = nil
	case tea.KeyEnter:
		m.rename = nil
		if name := string(r.text); name != "" {
			m.send(&api.Command{Op: api.OpRenameWindow, Target: r.window, Args: map[string]string{api.ArgName: name}})
		}
	default:
		if s, ok := keys.Edit(string(r.text), k); ok && utf8.RuneCountInString(s) <= maxWindowName {
			r.text = []rune(s)
		}
	}
}

// renamePrompt is the question shown in the status bar while renaming.
func (m *Model) renamePrompt() string {
	return "rename window: " + string(m.rename.text) + m.theme.Glyphs.Caret
}
