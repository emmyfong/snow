package server

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

type shutdownTestPane struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (*shutdownTestPane) State() api.SideState                { return noteState{} }
func (*shutdownTestPane) Act(string, map[string]string) error { return nil }
func (p *shutdownTestPane) Close() error {
	p.started <- struct{}{}
	<-p.release
	return nil
}

func TestCloseAllClosesPanesConcurrently(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }

	opts := testOptions()
	opts.sideKinds = []sideKind{{Name: "shutdown", Open: func(sideContext) (sidePane, error) {
		return &shutdownTestPane{started: started, release: release}, nil
	}}}
	s := New(opts)
	t.Cleanup(func() {
		releaseAll()
		s.Close()
	})
	if !s.call(func(st *state) {
		sess, err := st.createSession("shutdown", 80, 24, "")
		if err != nil {
			t.Errorf("create session: %v", err)
			return
		}
		for range 3 {
			if _, err := st.newWindow(sess, "shutdown", "", ""); err != nil {
				t.Errorf("create shutdown pane: %v", err)
				return
			}
		}
	}) {
		t.Fatal("server stopped before creating panes")
	}

	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("server did not start every pane close concurrently")
		}
	}
	releaseAll()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not finish shutdown after pane closes returned")
	}
}

func names(s *Server) []string {
	var out []string
	for _, info := range s.Sessions() {
		out = append(out, info.Name)
	}
	return out
}

func TestCreateListKill(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.CreateSession("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession("logs"); err != nil {
		t.Fatal(err)
	}
	if got := names(s); !slices.Equal(got, []string{"logs", "work"}) {
		t.Fatalf("sessions %q, want [logs work]", got)
	}
	if _, err := s.CreateSession("work"); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("duplicate create error = %v, want ErrSessionExists", err)
	}
	if err := s.KillSession("work"); err != nil {
		t.Fatal(err)
	}
	if err := s.KillSession("work"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("second kill error = %v, want ErrNoSession", err)
	}
	if got := names(s); !slices.Equal(got, []string{"logs"}) {
		t.Fatalf("sessions %q, want [logs]", got)
	}
}

func TestNumberedSessionNames(t *testing.T) {
	s := newTestServer(t)
	for _, want := range []string{"0", "1", "2"} {
		got, err := s.CreateSession("")
		if err != nil || got != want {
			t.Fatalf("CreateSession(\"\") = %q, %v; want %q", got, err, want)
		}
	}
	if err := s.KillSession("1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.CreateSession(""); got != "1" {
		t.Fatalf("next numbered session %q, want the free number 1", got)
	}
}

func TestPaneExitEndsSession(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.CreateSession("work"); err != nil {
		t.Fatal(err)
	}
	p := s.pane(t, "work")
	eventually(t, "prompt", func() bool { return screenHas(p, prompt()) })
	typeLine(p, "exit")
	eventually(t, "session to end", func() bool { return len(s.Sessions()) == 0 })
}

func TestServerDoneAfterLastSession(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession("b"); err != nil {
		t.Fatal(err)
	}
	if err := s.KillSession("a"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
		t.Fatal("server finished while session b still runs")
	default:
	}
	if err := s.KillSession("b"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "Done after the last session", func() bool {
		select {
		case <-s.Done():
			return true
		default:
			return false
		}
	})
}
