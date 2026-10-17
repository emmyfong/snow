package server

import (
	"slices"
	"time"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// endedNote is written under the output of a WSL pane that ended because
// its distribution stopped, for example after wsl --shutdown.
const endedNote = "\r\n\x1b[2m─── pane ended: WSL stopped, press r to restore ───\x1b[0m"

// wslStopped reports whether a pane running profile exited because its WSL
// distribution is no longer running. Typing exit leaves the distribution
// running, so that pane closes as usual, and so does a pane when WSL gives
// no answer. It starts wsl.exe, so it runs on the pane's exit goroutine,
// never on the model goroutine.
func (s *Server) wslStopped(profile string) bool {
	distroOf := s.launch.distro
	if s.opts.paneDistro != nil {
		distroOf = s.opts.paneDistro
	}
	if distroOf == nil {
		return false
	}
	distro, ok := distroOf(profile)
	if !ok {
		return false
	}
	if s.opts.distroRunning != nil {
		return !s.opts.distroRunning(distro)
	}
	running, err := shell.RunningDistros()
	if err != nil {
		s.log.Warn("could not ask WSL whether it runs; closing the pane", "err", err)
		return false
	}
	return !slices.Contains(running, distro)
}

// endPane keeps a pane whose WSL distribution stopped on screen, with a
// note that r restarts it.
func (st *state) endPane(sess *session, p *pane) {
	p.ended = true
	p.term.Print(endedNote)
	st.srv.log.Info("pane ended with WSL", "session", sess.name, "pane", p.id)
}

// endedKey handles a key sent to an ended pane: r restarts it, and every
// other key is dropped, as the pane has no program to take it. It reports
// whether the pane was an ended one.
func (st *state) endedKey(c *client, id int, key *api.Key) bool {
	sess, ok := st.sessions[c.session]
	if !ok {
		return false
	}
	w, p := sess.findPane(id)
	if p == nil || !p.ended {
		return false
	}
	if key != nil && key.Code == "r" && len(key.Mod) == 0 {
		st.restartPane(sess, w, p)
	}
	return true
}

// restartPane starts an ended pane again, in the same place and with the
// same ID: its profile in the folder it was last in, below its earlier
// lines and a divider.
func (st *state) restartPane(sess *session, w *window, old *pane) {
	cols, rows := sess.area()
	r := unzoomedRects(w.tree, cols, rows)[layout.PaneID(old.id)]
	p, err := st.srv.startPane(old.id, r.W, r.H, old.profile, old.folder(), preamble(lastLines(old, store.MaxScrollback), restoredDivider(time.Now())))
	if err != nil {
		st.srv.log.Warn("restart pane failed", "pane", old.id, "err", err)
		old.term.Print("\r\n\x1b[2mrestart failed: " + err.Error() + "\x1b[0m")
		return
	}
	w.panes[old.id] = p
	// The old program already exited; closing frees its emulator.
	st.closePane(old)
	st.repaint(sess)
	st.changed(sess)
}
