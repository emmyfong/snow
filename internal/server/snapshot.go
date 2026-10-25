package server

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/store"
)

// snapshot describes sess for the store: its windows, each one's layout
// and focus, and each pane's profile, folder, and last lines.
func (st *state) snapshot(sess *session) store.Snapshot {
	snap := store.Snapshot{Name: sess.name, Active: sess.active}
	for _, w := range sess.windows {
		sw := store.Window{Index: w.index, Name: w.name, Focused: w.focused}
		tree := terminalTree(w)
		if tree == nil {
			continue // only chats; they are not saved yet
		}
		if tree, err := json.Marshal(tree); err == nil {
			sw.Layout = tree
		} else {
			st.srv.log.Warn("save layout failed", "session", sess.name, "err", err)
		}
		for _, id := range tree.Panes() {
			p := w.panes[int(id)]
			if p == nil {
				continue
			}
			sw.Panes = append(sw.Panes, store.Pane{
				ID: p.id, Profile: p.profile, Folder: p.folder(), Scrollback: lastLines(p, st.srv.savedLines()),
			})
		}
		snap.Windows = append(snap.Windows, sw)
	}
	return snap
}

// terminalTree returns w's layout without its chat panes, which a restore
// cannot bring back yet, or nil when w holds only chats. A window without
// chats gives its own tree.
func terminalTree(w *window) *layout.Tree {
	var chats []layout.PaneID
	for id, p := range w.panes {
		if p.chat != nil {
			chats = append(chats, layout.PaneID(id))
		}
	}
	if len(chats) == 0 {
		return w.tree
	}
	if len(chats) == len(w.panes) {
		return nil
	}
	b, err := json.Marshal(w.tree)
	if err != nil {
		return nil
	}
	tree := &layout.Tree{}
	if err := json.Unmarshal(b, tree); err != nil {
		return nil
	}
	tree.Unzoom()
	for _, id := range chats {
		// A terminal pane is left, so no close is of the last pane.
		_ = tree.Close(id)
	}
	return tree
}

// savedLines is how many lines of each pane a snapshot keeps.
func (s *Server) savedLines() int {
	switch n := s.opts.SavedLines; {
	case n < 0:
		return 0
	case n == 0 || n > store.MaxScrollback:
		return store.MaxScrollback
	default:
		return n
	}
}

// lastLines returns the last n lines of p's history, without the empty rows
// below the screen's last text.
func lastLines(p *pane, n int) []string {
	if n <= 0 {
		return nil
	}
	_, total, _ := p.term.History(0, 0)
	lines, _, _ := p.term.History(max(total-n, 0), n)
	end := len(lines)
	for end > 0 && strings.TrimSpace(ansi.Strip(lines[end-1])) == "" {
		end--
	}
	return lines[:end]
}
