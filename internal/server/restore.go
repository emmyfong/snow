package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// loadSaved reads the snapshot of session when it is saved and not
// running, for an attach to restore. It reads the file outside the model
// goroutine, which must never wait for disk. It returns nil when there is
// nothing to restore.
func (s *Server) loadSaved(session string) (*store.Snapshot, error) {
	if s.store == nil {
		return nil, nil
	}
	saved := false
	s.call(func(st *state) {
		_, running := st.sessions[session]
		_, saved = st.saved[session]
		saved = saved && !running
	})
	if !saved {
		return nil, nil
	}
	snap, err := s.store.Load(session)
	if _, broken := errors.AsType[*store.DecodeError](err); broken {
		// The file broke after start-up: set it aside, as at start-up.
		note := s.setAside(err)
		s.call(func(st *state) {
			delete(st.saved, session)
			st.notes = append(st.notes, note)
			st.sessionsChanged()
		})
		return nil, errors.New(note)
	}
	if errors.Is(err, fs.ErrNotExist) {
		// Deleted outside Snow: forget it, and the attach finds no session.
		s.call(func(st *state) {
			delete(st.saved, session)
			st.sessionsChanged()
		})
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// restore rebuilds snap as a running session of cols by rows cells: its
// windows with their numbers, names, layouts, and focus, and each pane
// running its profile in its folder below its saved lines. Panes get new
// IDs. A profile that no longer exists runs the default; a folder that is
// gone falls back as for any new pane. A chat pane takes its conversation
// from chats, which resumeChats made off this goroutine.
func (st *state) restore(snap *store.Snapshot, chats map[int]*restoredChat, cols, rows int) (*session, error) {
	sess := &session{name: snap.Name, lastUsed: time.Now(), cols: cols, rows: rows}
	divider := restoredDivider(snap.Saved)
	for _, sw := range snap.Windows {
		w, err := st.restoreWindow(sess, sw, divider, chats)
		if err != nil {
			st.closeSession(sess)
			return nil, fmt.Errorf("restore %s: %w", snap.Name, err)
		}
		sess.windows = append(sess.windows, w)
	}
	if len(sess.windows) == 0 {
		if _, err := st.newWindow(sess, st.firstKind(), "", ""); err != nil {
			return nil, fmt.Errorf("restore %s: %w", snap.Name, err)
		}
	}
	slices.SortFunc(sess.windows, func(a, b *window) int { return a.index - b.index })
	sess.active = sess.windows[0].index
	if sess.window(snap.Active) != nil {
		sess.active = snap.Active
	}
	return sess, nil
}

// restoreWindow starts the panes of sw in a tree with new pane IDs.
func (st *state) restoreWindow(sess *session, sw store.Window, divider string, chats map[int]*restoredChat) (*window, error) {
	saved := map[int]store.Pane{}
	for _, p := range sw.Panes {
		saved[p.ID] = p
	}
	tree := &layout.Tree{}
	if err := json.Unmarshal(sw.Layout, tree); err != nil || tree.Root == nil {
		st.srv.log.Warn("saved layout unusable; restoring its first pane alone", "session", sess.name, "window", sw.Index, "err", err)
		first := 0
		if len(sw.Panes) > 0 {
			first = sw.Panes[0].ID
		}
		tree = layout.New(layout.PaneID(first))
	}
	ids := map[layout.PaneID]layout.PaneID{}  // old ID → new
	olds := map[layout.PaneID]layout.PaneID{} // new ID → old
	renumber(tree.Root, func(old layout.PaneID) layout.PaneID {
		id := layout.PaneID(st.nextPane)
		st.nextPane++
		ids[old], olds[id] = id, old
		return id
	})
	tree.Zoomed = ids[tree.Zoomed]
	w := &window{index: sw.Index, name: sw.Name, tree: tree, panes: map[int]*pane{}, focused: int(ids[layout.PaneID(sw.Focused)])}
	if sess.window(w.index) != nil {
		w.index = sess.freeIndex()
	}
	cols, rows := sess.area()
	rects := unzoomedRects(tree, cols, rows)
	// Start panes in layout order, so a failure part way is reproducible.
	for _, id := range tree.Panes() {
		sp := saved[int(olds[id])]
		r := rects[id]
		var p *pane
		var err error
		if rc := chats[sp.ID]; sp.Kind == store.KindChat && rc != nil && rc.conv != nil {
			p = rc.paneFor(int(id), sp.Profile)
		} else {
			p, err = st.srv.startPane(int(id), r.W, r.H, sp.Profile, sp.Folder, preamble(sp.Scrollback, divider))
			if errors.Is(err, ErrUnknownProfile) {
				p, err = st.srv.startPane(int(id), r.W, r.H, "", sp.Folder, preamble(sp.Scrollback, divider))
			}
		}
		if err != nil {
			for _, started := range w.panes {
				st.closePane(started)
			}
			return nil, err
		}
		w.panes[int(id)] = p
	}
	if w.panes[w.focused] == nil {
		w.focused = int(tree.Panes()[0])
	}
	if w.name == "" {
		w.name = w.panes[w.focused].profile
	}
	return w, nil
}

// renumber gives every leaf under n the ID next returns for its old one.
func renumber(n *layout.Node, next func(layout.PaneID) layout.PaneID) {
	if len(n.Children) == 0 {
		n.Pane = next(n.Pane)
		return
	}
	for _, c := range n.Children {
		renumber(c, next)
	}
}

// closeSession closes every pane of a session that never started fully.
func (st *state) closeSession(sess *session) {
	for _, w := range sess.windows {
		for _, p := range w.panes {
			st.closePane(p)
		}
	}
}

// restoredDivider is the line shown under a pane's saved lines.
func restoredDivider(saved time.Time) string {
	when := "an earlier session"
	if !saved.IsZero() {
		when = saved.Local().Format("Jan 2 15:04")
	}
	return "\x1b[2m─── restored from " + when + " ───\x1b[0m"
}

// preamble is what a restored pane shows before its program starts: the
// saved lines, each with its styles reset, then the divider.
func preamble(lines []string, divider string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\x1b[0m\r\n")
	}
	b.WriteString(divider)
	b.WriteString("\r\n")
	return b.String()
}

// savedList lists the saved sessions that are not running, for the home
// page.
func (st *state) savedList() []api.SessionInfo {
	var out []api.SessionInfo
	for name, info := range st.saved {
		if _, running := st.sessions[name]; running {
			continue
		}
		out = append(out, api.SessionInfo{Name: name, Windows: info.Windows, State: api.StateSaved, LastUsed: info.Saved})
	}
	return out
}

// unzoomedRects lays out every pane of tree as if no pane were zoomed. A
// zoomed window shows one pane, but the hidden ones still start at their
// real size: a pane started at 1x1 would wrap its restored lines to one
// cell each, and a resize does not join them again.
func unzoomedRects(tree *layout.Tree, cols, rows int) map[layout.PaneID]layout.Rect {
	return layout.Rects(&layout.Tree{Root: tree.Root}, cols, rows).Panes
}
