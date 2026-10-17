package main

import (
	"context"
	"github.com/charmbracelet/x/ansi"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// TestPersistenceRoundTrip is v1's promise end to end: server A builds a
// session of three panes in two folders, saves it, and stops; server B
// restores it with the same layout, folders, and earlier output.
func TestPersistenceRoundTrip(t *testing.T) {
	sessions, path := t.TempDir(), apitest.SocketPath(t)
	left, right := t.TempDir(), t.TempDir()

	// Server A.
	ctxA, stopA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)
	go func() {
		doneA <- server.Run(ctxA, server.Options{Version: "a", Spec: testShell, SessionsDir: sessions, IdleExit: time.Minute}, path)
	}()
	c := apitest.Dial(t, path, 160, 40)
	c.Send(&api.Attach{Session: "rt", Create: true, Dir: left})
	first := waitPanes(t, c, 1)[0].ID
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgFolder: right}})
	second := waitPanes(t, c, 2)[1].ID
	c.Send(&api.Command{Op: api.OpSplit, Target: second, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgFolder: right}})
	panes := waitPanes(t, c, 3)
	for i, p := range panes {
		c.Send(&api.Input{Pane: p.ID, Paste: "echo rt-mark-" + string(rune('a'+i)) + "\r"})
	}
	st, err := store.Open(sessions)
	if err != nil {
		t.Fatal(err)
	}
	// A rename is a change, saved 2 s later with the output so far; the
	// periodic save would take 30 s. Rename until a save holds every marker.
	deadline := time.Now().Add(60 * time.Second)
	for i := 0; ; i++ {
		if snap, err := st.Load("rt"); err == nil && allMarks(snap) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the snapshot never held all three panes' output")
		}
		c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: "rt" + string(rune('0'+i%10))}})
		time.Sleep(2500 * time.Millisecond)
	}
	c.Close()
	stopA()
	if err := <-doneA; err != nil {
		t.Fatalf("server A: %v", err)
	}

	// Server B, recording where it starts each pane.
	var mu sync.Mutex
	var dirs []string
	spec := func(profile, dir string) (term.Spec, string, error) {
		s, name, err := testShell(profile, dir)
		if dir != "" {
			mu.Lock()
			dirs = append(dirs, dir)
			mu.Unlock()
		}
		return s, name, err
	}
	ctxB, stopB := context.WithCancel(context.Background())
	doneB := make(chan error, 1)
	go func() {
		doneB <- server.Run(ctxB, server.Options{Version: "b", Spec: spec, SessionsDir: sessions, IdleExit: time.Minute}, path)
	}()
	t.Cleanup(func() {
		stopB()
		if err := <-doneB; err != nil {
			t.Errorf("server B: %v", err)
		}
	})
	d := apitest.Dial(t, path, 160, 40)
	d.Send(&api.Attach{Session: "rt"})
	got := waitPanes(t, d, 3)

	// The same shape: pane 1 on the left, panes 2 and 3 stacked on the right.
	a, b, z := got[0].Rect, got[1].Rect, got[2].Rect
	if a.X >= b.X || b.X != z.X || b.Y >= z.Y || a.H <= b.H {
		t.Fatalf("restored rects %+v %+v %+v, want h(1, v(2, 3))", a, b, z)
	}
	mu.Lock()
	gotDirs := slices.Clone(dirs)
	mu.Unlock()
	if want := []string{left, right, right}; !slices.Equal(gotDirs, want) {
		t.Fatalf("restored panes started in %v, want %v", gotDirs, want)
	}
	// Each pane's history holds its earlier output above the divider. The
	// history, not the screen: on Windows the shell's first clear moves the
	// restored lines into scrollback.
	for _, p := range got {
		text := history(t, d, p.ID, "restored")
		if mark, div := strings.Index(text, "rt-mark-"), strings.Index(text, "restored"); mark < 0 || div < mark {
			t.Fatalf("pane %d history lost its earlier output:\n%s", p.ID, text)
		}
	}
}

// history reads pane id's history over the protocol, as copy mode does,
// until it contains want, and returns it as plain text.
func history(t *testing.T, c *apitest.Client, id int, want string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		c.Send(&api.ScrollbackRequest{Pane: id, From: 0, Count: api.MaxScrollbackLines})
		for {
			m, err := c.Next(time.Second)
			if err != nil {
				break
			}
			if sb, ok := m.(*api.Scrollback); ok && sb.Pane == id {
				text := ansi.Strip(strings.Join(sb.Lines, "\n"))
				if strings.Contains(text, want) {
					return text
				}
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pane %d history never showed %q", id, want)
	return ""
}

// waitPanes reads until a Layout shows n panes in the shown window, and
// returns them.
func waitPanes(t *testing.T, c *apitest.Client, n int) []api.PaneInfo {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m, _ := c.Next(time.Second)
		if l, ok := m.(*api.Layout); ok {
			for _, w := range l.Windows {
				if w.Index == l.Active && len(w.Panes) == n {
					return w.Panes
				}
			}
		}
	}
	t.Fatalf("never saw %d panes", n)
	return nil
}

// allMarks reports whether every pane of snap saved its marker line.
func allMarks(snap store.Snapshot) bool {
	n := 0
	for _, w := range snap.Windows {
		for _, p := range w.Panes {
			if strings.Contains(strings.Join(p.Scrollback, "\n"), "rt-mark-") {
				n++
			}
		}
	}
	return n == 3
}
