package server

import (
	"encoding/json"
	"github.com/charmbracelet/x/ansi"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// restoreServer starts a server over dir, where earlier snapshots wait.
// dirs records the folder each pane was started in.
func restoreServer(t *testing.T, dir string) (*Server, *startedDirs) {
	t.Helper()
	opts := testOptions()
	opts.SessionsDir = dir
	opts.saveDelay = 20 * time.Millisecond
	opts.busyEvery = time.Hour
	dirs := &startedDirs{}
	spec := opts.Spec
	opts.Spec = func(profile, d string) (term.Spec, string, error) {
		s, name, err := spec(profile, d)
		if d != "" || profile != "" {
			dirs.add(s.Dir) // the launcher's own probe passes neither
		}
		return s, name, err
	}
	s := New(opts)
	t.Cleanup(s.Close)
	return s, dirs
}

type startedDirs struct {
	mu   sync.Mutex
	dirs []string
}

func (d *startedDirs) add(dir string) { d.mu.Lock(); d.dirs = append(d.dirs, dir); d.mu.Unlock() }
func (d *startedDirs) list() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.dirs...)
}

// writeSnapshot saves snap in dir, as an earlier server would have.
func writeSnapshot(t *testing.T, dir string, snap store.Snapshot) {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(snap); err != nil {
		t.Fatal(err)
	}
}

// twoPanes is a saved session "api": one window split into panes 7 and 9.
func twoPanes(t *testing.T, folder string, lines ...string) store.Snapshot {
	t.Helper()
	tree := layout.New(7)
	if err := tree.Split(7, layout.Right, 9); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	return store.Snapshot{Name: "api", Windows: []store.Window{{
		Index: 2, Name: "dev", Layout: raw, Focused: 9,
		Panes: []store.Pane{{ID: 7, Profile: "sh", Folder: folder, Scrollback: lines}, {ID: 9, Profile: "no-such-profile"}},
	}}}
}

func TestSavedSessionsAreListed(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, twoPanes(t, ""))
	s, _ := restoreServer(t, dir)
	c := apitest.New(t, dialPipe(s), 80, 24)
	m, _ := c.Next(5 * time.Second)
	l, ok := m.(*api.SessionList)
	if !ok || len(l.Sessions) != 1 || l.Sessions[0].Name != "api" || l.Sessions[0].State != api.StateSaved || l.Sessions[0].Windows != 1 {
		t.Fatalf("first message %#v, want api listed as saved", m)
	}
}

// TestRestoreLayout: attaching to a saved session rebuilds its windows with
// the same tree, window number, name, and focus, and starts every pane; a
// profile that no longer exists starts the default.
func TestRestoreLayout(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, twoPanes(t, ""))
	s, _ := restoreServer(t, dir)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "api"})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	if l.Session != "api" || len(l.Windows) != 1 || l.Windows[0].Index != 2 || l.Windows[0].Name != "dev" || len(l.Windows[0].Panes) != 2 {
		t.Fatalf("layout %+v, want window 2 'dev' with two panes", l)
	}
	left, right := l.Windows[0].Panes[0], l.Windows[0].Panes[1]
	if left.Rect.X >= right.Rect.X || left.Rect.Y != right.Rect.Y || !right.Focused {
		t.Fatalf("panes %+v and %+v, want side by side with the right one focused", left, right)
	}
	if right.Profile == "no-such-profile" || right.Profile == "" {
		t.Fatalf("pane with an unknown profile runs %q, want the default", right.Profile)
	}
}

func TestRestoreFolderFallback(t *testing.T) {
	dir := t.TempDir()
	here := t.TempDir()
	gone := filepath.Join(t.TempDir(), "deleted")
	snap := twoPanes(t, here)
	snap.Windows[0].Panes[1].Folder = gone
	writeSnapshot(t, dir, snap)
	s, dirs := restoreServer(t, dir)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "api"})
	nextLayout(t, c, func(*api.Layout) bool { return true })
	home, _ := os.UserHomeDir()
	got := dirs.list()
	if len(got) != 2 || got[0] != here || got[1] != home {
		t.Fatalf("panes started in %v, want %s, then home (%s) for the missing folder", got, here, home)
	}
}

// TestRestoredDivider: the saved lines come back above a divider, and the
// new shell's prompt follows it.
func TestRestoredDivider(t *testing.T) {
	dir := t.TempDir()
	// One pane: the test client keeps one set of rows for every pane.
	writeSnapshot(t, dir, store.Snapshot{Name: "api", Windows: []store.Window{{
		Name: "sh", Panes: []store.Pane{{ID: 1, Profile: "sh", Scrollback: []string{"line from before the reboot"}}},
	}}})
	s, _ := restoreServer(t, dir)
	c := connect(t, s, 160, 24)
	c.Send(&api.Attach{Session: "api"})
	nextLayout(t, c, func(*api.Layout) bool { return true })
	// History, not the screen: on Windows the shell's first clear moves the
	// restored lines into scrollback.
	var text string
	eventually(t, "divider in the history", func() bool {
		text = paneHistory(s, "api")
		return strings.Contains(text, "restored")
	})
	before, divider := strings.Index(text, "line from before"), strings.Index(text, "restored")
	if before < 0 || divider < before {
		t.Fatalf("history:\n%s\nwant the saved line above the divider", text)
	}
}

func TestKillSavedSession(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, twoPanes(t, ""))
	s, _ := restoreServer(t, dir)
	c := connect(t, s, 80, 24)
	c.Send(&api.KillSession{Session: "api"})
	nextList(t, c, func(l *api.SessionList) bool { return len(l.Sessions) == 0 })
	st, _ := store.Open(dir)
	eventually(t, "file deleted", func() bool { infos, _ := st.List(); return len(infos) == 0 })
}

// dialPipe connects to s over an in-memory pipe and returns the client's end.
func dialPipe(s *Server) net.Conn {
	a, b := net.Pipe()
	go s.serveConn(b)
	return a
}

func TestRenameSavedSession(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, twoPanes(t, ""))
	s, _ := restoreServer(t, dir)
	c := connect(t, s, 80, 24)
	c.Send(&api.RenameSession{Session: "api", Name: "backend"})
	nextList(t, c, func(l *api.SessionList) bool { return hasSession(l, "backend", 0) })
	st, _ := store.Open(dir)
	eventually(t, "file renamed", func() bool {
		infos, _ := st.List()
		return len(infos) == 1 && infos[0].Name == "backend"
	})
	c.Send(&api.Attach{Session: "backend"})
	if l := nextLayout(t, c, func(*api.Layout) bool { return true }); l.Session != "backend" || len(l.Windows[0].Panes) != 2 {
		t.Fatalf("layout %+v, want backend restored with both panes", l)
	}
}

// TestServerStartsWithBrokenFile: a broken snapshot does not stop the
// server or hide the good ones; it moves aside and the session list says
// where it went.
func TestServerStartsWithBrokenFile(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, twoPanes(t, ""))
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := restoreServer(t, dir)
	c := apitest.New(t, dialPipe(s), 80, 24)
	m, _ := c.Next(5 * time.Second)
	l, ok := m.(*api.SessionList)
	if !ok || len(l.Sessions) != 1 || l.Sessions[0].Name != "api" {
		t.Fatalf("first message %#v, want api listed", m)
	}
	if len(l.Broken) != 1 || !strings.Contains(l.Broken[0], "bad.json") || !strings.Contains(l.Broken[0], filepath.Join(dir, "broken")) {
		t.Fatalf("broken notes %q, want one naming bad.json and where it moved", l.Broken)
	}
}

// paneHistory returns the focused pane's whole history in session as plain
// text: its scrollback, then its screen.
func paneHistory(s *Server, session string) string {
	var text string
	s.call(func(st *state) {
		sess, ok := st.sessions[session]
		if !ok {
			return
		}
		w := sess.shown()
		p := w.panes[w.focused]
		_, total, _ := p.term.History(0, 0)
		lines, _, _ := p.term.History(0, total)
		text = ansi.Strip(strings.Join(lines, "\n"))
	})
	return text
}
