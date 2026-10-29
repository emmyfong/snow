package server

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/store"
)

// snapshot describes sess for the store: its windows, each one's layout
// and focus, and each pane's kind, profile, and folder, with a terminal's
// last lines or a chat's conversation ID.
func (st *state) snapshot(sess *session) store.Snapshot {
	snap := store.Snapshot{Name: sess.name, Active: sess.active}
	for _, w := range sess.windows {
		sw := store.Window{Index: w.index, Name: w.name, Focused: w.focused}
		if tree, err := json.Marshal(w.tree); err == nil {
			sw.Layout = tree
		} else {
			st.srv.log.Warn("save layout failed", "session", sess.name, "err", err)
		}
		for _, id := range w.tree.Panes() {
			p := w.panes[int(id)]
			if p == nil {
				continue
			}
			sp := store.Pane{ID: p.id, Profile: p.profile, Folder: p.folder()}
			if p.chat != nil {
				// The conversation file holds the transcript.
				sp.Kind, sp.Conversation = store.KindChat, p.chat.conv.ID()
			} else {
				sp.Scrollback = lastLines(p, st.srv.savedLines())
			}
			sw.Panes = append(sw.Panes, sp)
		}
		snap.Windows = append(snap.Windows, sw)
	}
	return snap
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
