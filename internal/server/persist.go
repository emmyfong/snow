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
// their last save are saved too.
const (
	defaultSaveDelay = 2 * time.Second
	defaultBusyEvery = 30 * time.Second
)

// openSaver returns the saver for opts.SessionsDir, or nil when the server
// does not save sessions. A folder it cannot open turns saving off, with a
// warning: sessions still run, they are only not kept.
func (s *Server) openSaver() *store.Saver {
	if s.opts.SessionsDir == "" {
		return nil
	}
	st, err := store.Open(s.opts.SessionsDir)
	if err != nil {
		s.log.Warn("sessions will not be saved", "err", err)
		return nil
	}
	s.store = st
	infos, errs := st.List()
	s.startSaved = infos
	for _, err := range errs {
		s.startBroken = append(s.startBroken, s.setAside(err))
	}
	delay := cmpDuration(s.opts.saveDelay, defaultSaveDelay)
	return store.NewSaver(st, delay, func(session string, err error) {
		s.log.Warn("save failed", "session", session, "err", err)
	})
}

func cmpDuration(d, def time.Duration) time.Duration {
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
	st.savedAt[sess.name] = contentVersion(sess)
	snap := st.snapshot(sess)
	st.saved[sess.name] = store.Info{Name: sess.name, Saved: time.Now(), Windows: len(snap.Windows)}
	st.srv.saver.Changed(snap)
}

// unsave removes the saved file of session, for a session that was killed
// or renamed. The delete runs off the model goroutine: it waits for disk.
// A save the saver has pending for the name is dropped first, and a later
// save of the same name waits at least the save delay, so it lands after.
func (st *state) unsave(session string) {
	delete(st.savedAt, session)
	delete(st.saved, session)
	saver := st.srv.saver
	if saver == nil {
		return
	}
	log := st.srv.log
	go func() {
		if err := saver.Delete(session); err != nil {
			log.Warn("delete saved session failed", "session", session, "err", err)
		}
	}()
}

// saveBusy saves the sessions whose panes printed since their last save.
func (st *state) saveBusy() {
	for name, sess := range st.sessions {
		if contentVersion(sess) != st.savedAt[name] {
			st.save(sess)
		}
	}
}

// saveAll writes every session now, for a clean exit such as a restart.
func (st *state) saveAll() {
	if st.srv.saver == nil {
		return
	}
	for _, sess := range st.sessions {
		st.srv.saver.Changed(st.snapshot(sess))
	}
	if err := st.srv.saver.Flush(); err != nil {
		st.srv.log.Warn("save on exit failed", "err", err)
	}
}

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
				ID: p.id, Profile: p.profile, Folder: p.term.Folder(), Scrollback: lastLines(p),
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
