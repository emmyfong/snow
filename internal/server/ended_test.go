package server

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// wslServer is a test server whose panes count as WSL panes; running says
// whether their distribution still runs when a pane exits.
func wslServer(t *testing.T, running bool) *Server {
	t.Helper()
	opts := testOptions()
	opts.paneDistro = func(string) (string, bool) { return "Ubuntu", true }
	opts.distroRunning = func(string) bool { return running }
	s := New(opts)
	t.Cleanup(s.Close)
	return s
}

// splitTwo attaches c to a session and splits it into two panes, returning
// the first pane's ID and the second's.
func splitTwo(t *testing.T, s *Server) (*apitest.Client, int, int) {
	t.Helper()
	c, first := attached(t, s, 120, 25)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == 2 })
	second := l.Windows[0].Panes[1].ID
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	return c, first, second
}

// TestRestoreEndedPane: a WSL pane that ended because WSL stopped stays on
// screen and says so; r restarts that pane alone.
func TestRestoreEndedPane(t *testing.T) {
	s := wslServer(t, false)
	c, first, second := splitTwo(t, s)
	c.Send(&api.Input{Pane: first, Paste: "echo before" + "stop\r"})
	c.WaitScreen("beforestop")
	c.Send(&api.Input{Pane: first, Paste: "exit\r"})
	c.WaitScreen("press r to restore")
	c.Send(&api.Input{Pane: first, Key: &api.Key{Code: "r", Text: "r"}})
	// History, not the screen: on Windows the new shell's first clear moves
	// the restored lines into scrollback.
	eventually(t, "restarted pane's divider", func() bool {
		var text string
		s.call(func(st *state) {
			_, p := st.sessions["work"].findPane(first)
			_, total, _ := p.term.History(0, 0)
			lines, _, _ := p.term.History(0, total)
			text = strings.Join(lines, "\n")
		})
		return strings.Contains(text, "restored") && strings.Contains(text, "beforestop")
	})
	var ended []bool
	s.call(func(st *state) {
		w := st.sessions["work"].shown()
		for _, id := range []int{first, second} {
			if p := w.panes[id]; p != nil {
				ended = append(ended, p.ended)
			}
		}
	})
	if len(ended) != 2 || ended[0] || ended[1] {
		t.Fatalf("panes %d and %d: ended %v, want both there and running", first, second, ended)
	}
}

// TestExitClosesPaneWhileDistroRuns: typing exit in a WSL pane whose
// distribution still runs closes the pane, as for any shell.
func TestExitClosesPaneWhileDistroRuns(t *testing.T) {
	s := wslServer(t, true)
	c, first, _ := splitTwo(t, s)
	c.Send(&api.Input{Pane: first, Paste: "exit\r"})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == 1 })
}

// TestClosedPaneSkipsWSLCheck: a pane Snow closes did not end with WSL, so
// its exit starts no wsl.exe query.
func TestClosedPaneSkipsWSLCheck(t *testing.T) {
	opts := testOptions()
	opts.paneDistro = func(string) (string, bool) { return "Ubuntu", true }
	var asked atomic.Int32
	opts.distroRunning = func(string) bool { asked.Add(1); return true }
	s := New(opts)
	t.Cleanup(s.Close)
	c, first, _ := splitTwo(t, s)
	c.Send(&api.Command{Op: api.OpClose, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows[0].Panes) == 1 })
	time.Sleep(200 * time.Millisecond) // the exit goroutine runs after the close
	if n := asked.Load(); n != 0 {
		t.Fatalf("closing a pane asked WSL %d times, want 0", n)
	}
}
