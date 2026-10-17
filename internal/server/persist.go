package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/store"
)

// Save timing. A session is saved saveDelay after its last change to
// windows, panes, or layout. Output is not a change, or a busy pane would
// never save; instead, every busyEvery, sessions whose panes printed since
// their last save are saved too. A session that ends because its last pane
// exited has its file deleted endDelay later, not at once: at an OS
// shutdown the shells can die before the server's signal arrives, and the
// session must not be deleted then.
const (
	defaultSaveDelay = 2 * time.Second
	defaultBusyEvery = 30 * time.Second
	defaultEndDelay  = 5 * time.Second
)

// notSaving starts the note shown on the home page while saving fails.
const notSaving = "Sessions are not being saved: "

// openSaver returns the saver for opts.SessionsDir, or nil when the server
// does not save sessions. A folder it cannot open turns saving off: sessions
// still run, and the home page says they are not kept.
func (s *Server) openSaver() *store.Saver {
	if s.opts.SessionsDir == "" {
		return nil
	}
	st, err := store.Open(s.opts.SessionsDir)
	if err != nil {
		s.log.Warn("sessions will not be saved", "err", err)
		s.startNotes = append(s.startNotes, notSaving+err.Error())
		return nil
	}
	s.store = st
	infos, errs := st.List()
	s.startSaved = infos
	for _, err := range errs {
		s.startNotes = append(s.startNotes, s.setAside(err))
	}
	delay := orDefault(s.opts.saveDelay, defaultSaveDelay)
	return store.NewSaver(st, delay, func(session string, err error) {
		s.post(func(st *state) { st.saveResult(session, err) })
	})
}

func orDefault(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// save schedules sess to be written. The snapshot is taken now, in the
// model goroutine; the write happens later on the saver's own goroutine.
func (st *state) save(sess *session) {
	if st.srv.saver == nil {
		return
	}
	st.stopEndTimer(sess.name)
	st.savedAt[sess.name] = contentVersion(sess)
	snap := st.snapshot(sess)
	st.saved[sess.name] = store.Info{Name: sess.name, Saved: time.Now(), Windows: len(snap.Windows)}
	st.srv.saver.Changed(snap)
}

// saveResult records how a save or delete went. A failure shows on the home
// page and marks the session unsaved, so the next periodic save tries again;
// a success clears the note.
func (st *state) saveResult(session string, err error) {
	before := st.saveProblem
	if err != nil {
		st.srv.log.Warn("save failed", "session", session, "err", err)
		st.saveProblem = notSaving + err.Error()
		delete(st.savedAt, session)
	} else {
		st.saveProblem = ""
	}
	if st.saveProblem != before {
		st.sessionsChanged()
	}
}

// unsave removes the saved file of session now, for a session that was
// killed or renamed. The saver drops any save pending for it and deletes
// the file on its own goroutine, after the saves asked for earlier.
func (st *state) unsave(session string) {
	st.stopEndTimer(session)
	delete(st.savedAt, session)
	delete(st.saved, session)
	if st.srv.saver != nil {
		st.srv.saver.Delete(session)
	}
}

// unsaveLater removes the saved file of a session that ended on its own,
// endDelay from now, unless a session of that name runs again by then or
// the server stops by signal first.
func (st *state) unsaveLater(session string) {
	if st.srv.saver == nil {
		return
	}
	st.stopEndTimer(session)
	srv := st.srv
	var t *time.Timer
	t = time.AfterFunc(orDefault(srv.opts.endDelay, defaultEndDelay), func() {
		srv.post(func(st *state) {
			if st.endTimers[session] == t {
				st.unsave(session)
			}
		})
	})
	st.endTimers[session] = t
}

func (st *state) stopEndTimer(session string) {
	if t := st.endTimers[session]; t != nil {
		t.Stop()
		delete(st.endTimers, session)
	}
}

// saveBusy saves the sessions whose panes printed since their last save.
func (st *state) saveBusy() {
	for name, sess := range st.sessions {
		if contentVersion(sess) != st.savedAt[name] {
			st.save(sess)
		}
	}
}

// stopSaving finishes saving as the server stops. A normal stop, such as a
// restart or the last session ending, writes every session and runs the
// deletes still waiting. A stop by signal, as at an OS shutdown, leaves the
// disk as it is: the shells may already have died and taken panes and
// sessions with them, and the last saves hold the state to restore.
func (st *state) stopSaving() {
	saver := st.srv.saver
	if saver == nil {
		return
	}
	if st.srv.keepDisk.Load() {
		for name := range st.endTimers {
			st.stopEndTimer(name)
		}
		saver.Abandon()
		return
	}
	for name := range st.endTimers {
		st.unsave(name)
	}
	for _, sess := range st.sessions {
		saver.Changed(st.snapshot(sess))
	}
	if err := saver.Close(); err != nil {
		st.srv.log.Warn("save on exit failed", "err", err)
	}
}

// keepDiskOnClose makes the next Close leave saved sessions as they are on
// disk, for a stop by signal.
func (s *Server) keepDiskOnClose() { s.keepDisk.Store(true) }

// contentVersion changes whenever a pane of sess prints, so saveBusy can
// tell a session that has new output from one that does not.
func contentVersion(sess *session) uint64 {
	var v uint64
	for _, w := range sess.windows {
		for _, p := range w.panes {
			v += p.term.Version()
		}
	}
	return v
}

// snapshot describes sess for the store: its windows, each one's layout
// and focus, and each pane's profile, folder, and last lines.
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
			sw.Panes = append(sw.Panes, store.Pane{
				ID: p.id, Profile: p.profile, Folder: p.folder(), Scrollback: lastLines(p),
			})
		}
		snap.Windows = append(snap.Windows, sw)
	}
	return snap
}

// lastLines returns the last store.MaxScrollback lines of p's history,
// without the empty rows below the screen's last text.
func lastLines(p *pane) []string {
	_, total, _ := p.term.History(0, 0)
	lines, _, _ := p.term.History(max(total-store.MaxScrollback, 0), store.MaxScrollback)
	end := len(lines)
	for end > 0 && strings.TrimSpace(ansi.Strip(lines[end-1])) == "" {
		end--
	}
	return lines[:end]
}

// savedInfos returns the sessions saved when the server started, by name.
func (s *Server) savedInfos() map[string]store.Info {
	out := make(map[string]store.Info, len(s.startSaved))
	for _, info := range s.startSaved {
		out[info.Name] = info
	}
	return out
}

// setAside reports a saved session that cannot be read: a broken file moves
// to the broken folder. It returns a note for the home page.
func (s *Server) setAside(err error) string {
	s.log.Warn("saved session unreadable", "err", err)
	de, ok := errors.AsType[*store.DecodeError](err)
	if !ok {
		return err.Error()
	}
	moved, merr := s.store.MoveBroken(de.Path, time.Now())
	if merr != nil {
		s.log.Warn("could not move broken snapshot", "err", merr)
		return fmt.Sprintf("%s cannot be read (%v) or moved (%v)", de.Path, de.Err, merr)
	}
	return fmt.Sprintf("%s cannot be read (%v); moved to %s", de.Path, de.Err, moved)
}
