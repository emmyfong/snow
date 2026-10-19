package server

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// persistServer returns a server that saves sessions in a temporary folder,
// with short delays so tests run fast, and that folder's store.
func persistServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = 20 * time.Millisecond
	opts.busyEvery = 100 * time.Millisecond
	s := New(opts)
	t.Cleanup(s.Close)
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

// saved waits until session's snapshot satisfies ok and returns it.
func saved(t *testing.T, st *store.Store, session string, ok func(store.Snapshot) bool) store.Snapshot {
	t.Helper()
	var snap store.Snapshot
	eventually(t, "snapshot of "+session, func() bool {
		var err error
		snap, err = st.Load(session)
		return err == nil && ok(snap)
	})
	return snap
}

func TestChangeIsSaved(t *testing.T) {
	s, st := persistServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	snap := saved(t, st, "a", func(store.Snapshot) bool { return true })
	if len(snap.Windows) != 1 || len(snap.Windows[0].Panes) != 1 || snap.Windows[0].Panes[0].Profile != "sh" && snap.Windows[0].Panes[0].Profile != "cmd" {
		t.Fatalf("snapshot %+v, want one window with one shell pane", snap)
	}
	if len(snap.Windows[0].Layout) == 0 {
		t.Fatal("the snapshot has no layout")
	}
}

func TestKillDeletesSnapshot(t *testing.T) {
	s, st := persistServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	saved(t, st, "a", func(store.Snapshot) bool { return true })
	if err := s.KillSession("a"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "snapshot deleted", func() bool {
		_, err := st.Load("a")
		return errors.Is(err, fs.ErrNotExist)
	})
}

// TestBusySessionIsSaved: output alone is not a change, but the periodic
// save catches it, so a reboot keeps recent output.
func TestBusySessionIsSaved(t *testing.T) {
	s, st := persistServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Input{Pane: 1, Paste: "echo busy" + "mark\r"})
	saved(t, st, "work", func(snap store.Snapshot) bool {
		return strings.Contains(strings.Join(snap.Windows[0].Panes[0].Scrollback, "\n"), "busymark")
	})
}

// TestCloseSavesEverySession: a clean exit, such as a restart, writes every
// session at once, without waiting for the save delay.
func TestCloseSavesEverySession(t *testing.T) {
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = time.Hour
	opts.busyEvery = time.Hour
	s := New(opts)
	for _, name := range []string{"a", "b"} {
		if _, err := s.CreateSession(name); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := st.Load(name); err != nil {
			t.Errorf("session %s not saved on close: %v", name, err)
		}
	}
}

// TestSavedFolderWithoutShellReports: a shell that never reports its folder
// (no OSC 7) is saved in the folder it started in.
func TestSavedFolderWithoutShellReports(t *testing.T) {
	s, st := persistServer(t)
	here := t.TempDir()
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true, Dir: here})
	snap := saved(t, st, "work", func(snap store.Snapshot) bool { return len(snap.Windows) == 1 })
	if got := snap.Windows[0].Panes[0].Folder; got != here {
		t.Fatalf("saved folder %q, want %q, where the pane started", got, here)
	}
}

// endTestServer is a persisting server whose sessions save at once, with a
// long wait before an ended session's file is deleted.
func endTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = 10 * time.Millisecond
	opts.busyEvery = time.Hour
	opts.endDelay = time.Hour
	s := New(opts)
	t.Cleanup(s.Close)
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

// endLastPane makes session work's only pane exit, which ends the session.
func endLastPane(t *testing.T, s *Server, st *store.Store) {
	t.Helper()
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	saved(t, st, "work", func(store.Snapshot) bool { return true })
	c.Send(&api.Input{Pane: 1, Paste: "exit\r"})
	// Sessions also lists saved ones, and the ended session's file is still
	// there; count the running ones.
	eventually(t, "session ended", func() bool {
		running := 0
		s.call(func(st *state) { running = len(st.sessions) })
		return running == 0
	})
}

// TestSignalKeepsEndedSessions: at an OS shutdown the shells can die before
// the server's signal arrives; the session then looks ended. A stop by
// signal keeps the files as they are, so the reboot restores them.
func TestSignalKeepsEndedSessions(t *testing.T) {
	s, st := endTestServer(t)
	endLastPane(t, s, st)
	s.keepDiskOnClose()
	s.Close()
	if _, err := st.Load("work"); err != nil {
		t.Fatalf("a stop by signal deleted the ended session: %v", err)
	}
}

// TestNormalStopDeletesEndedSessions: exit in the last pane ends the
// session for good; a normal stop deletes its file at once.
func TestNormalStopDeletesEndedSessions(t *testing.T) {
	s, st := endTestServer(t)
	endLastPane(t, s, st)
	s.Close()
	if _, err := st.Load("work"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an ended session's file survived a normal stop: %v", err)
	}
}

// TestEndedSessionNotOffered: an ended session's file waits endDelay before
// it goes, but the home page its client lands on must not offer it as
// saved, and a new session of that name starts fresh, not restored.
func TestEndedSessionNotOffered(t *testing.T) {
	s, st := endTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: "old"}})
	saved(t, st, "work", func(snap store.Snapshot) bool { return snap.Windows[0].Name == "old" })
	c.Send(&api.Input{Pane: 1, Paste: "exit\r"})
	if e := nextError(t, c); e.Code != api.CodeSessionEnded {
		t.Fatalf("got %+v, want CodeSessionEnded", e)
	}
	l := nextList(t, c, func(*api.SessionList) bool { return true })
	if slices.ContainsFunc(l.Sessions, func(i api.SessionInfo) bool { return i.Name == "work" }) {
		t.Fatalf("the home page offers the ended session: %+v", l.Sessions)
	}
	c.Send(&api.Attach{Session: "work", Create: true})
	if got := nextLayout(t, c, func(*api.Layout) bool { return true }); got.Windows[0].Name == "old" {
		t.Fatal("a new session named work restored the ended one")
	}
}

// TestSavedLinesNone: with no lines to keep, a snapshot holds the layout
// and folders but none of the panes' output.
func TestSavedLinesNone(t *testing.T) {
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.SavedLines = -1
	opts.saveDelay = 10 * time.Millisecond
	s := New(opts)
	t.Cleanup(s.Close)
	st, _ := store.Open(opts.SessionsDir)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Input{Pane: 1, Paste: "echo secret-value\r"})
	c.WaitScreen("secret-value\n")
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: "x"}})
	snap := saved(t, st, "work", func(snap store.Snapshot) bool { return snap.Windows[0].Name == "x" })
	if lines := snap.Windows[0].Panes[0].Scrollback; len(lines) != 0 || len(snap.Windows[0].Layout) == 0 {
		t.Fatalf("saved lines %q, layout %d bytes; want no lines and a layout", lines, len(snap.Windows[0].Layout))
	}
}
