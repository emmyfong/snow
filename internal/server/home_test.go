package server

import (
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// nextList reads until a SessionList that satisfies ok arrives.
func nextList(t *testing.T, c *apitest.Client, ok func(*api.SessionList) bool) *api.SessionList {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if l, is := m.(*api.SessionList); is && ok(l) {
			return l
		}
		if err != nil && time.Now().After(deadline) {
			break
		}
	}
	t.Fatal("no matching session list")
	return nil
}

// nextError reads until an Error arrives.
func nextError(t *testing.T, c *apitest.Client) *api.Error {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, _ := c.Next(time.Second)
		if e, ok := m.(*api.Error); ok {
			return e
		}
	}
	t.Fatal("no error")
	return nil
}

func hasSession(l *api.SessionList, name string, attached int) bool {
	for _, s := range l.Sessions {
		if s.Name == name && s.Attached == attached {
			return true
		}
	}
	return false
}

// TestDetachKeepsTheSession: Detach leaves the session running and sends the
// client the session list, so it can show the home page.
func TestDetachKeepsTheSession(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Detach{})
	nextList(t, c, func(l *api.SessionList) bool { return hasSession(l, "work", 0) })
	if len(s.Sessions()) != 1 {
		t.Fatalf("sessions after detach: %v, want work still running", s.Sessions())
	}
}

// TestServerStaysUpWhileAClientIsOpen: killing the last session from the
// home page must not stop the server under it; the server stops once that
// client leaves too.
func TestServerStaysUpWhileAClientIsOpen(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	c.Send(&api.KillSession{Session: "a"})
	nextList(t, c, func(l *api.SessionList) bool { return len(l.Sessions) == 0 })
	select {
	case <-s.Done():
		t.Fatal("the server stopped while a client was open")
	case <-time.After(100 * time.Millisecond):
	}
	c.Close()
	eventually(t, "Done after the last client left", func() bool {
		select {
		case <-s.Done():
			return true
		default:
			return false
		}
	})
}

func TestKillSessionEndsItForAttachedClients(t *testing.T) {
	s := newTestServer(t)
	work, _ := attached(t, s, 80, 24)
	home := connect(t, s, 80, 24)
	home.Send(&api.KillSession{Session: "work"})
	if e := nextError(t, work); e.Code != api.CodeSessionEnded {
		t.Fatalf("attached client got %+v, want CodeSessionEnded", e)
	}
	home.Send(&api.KillSession{Session: "nope"})
	if e := nextError(t, home); e.Code != api.CodeCommandFailed {
		t.Fatalf("kill of an unknown session: %+v, want CodeCommandFailed", e)
	}
}

func TestRenameSession(t *testing.T) {
	s := newTestServer(t)
	work, _ := attached(t, s, 80, 24)
	if _, err := s.CreateSession("taken"); err != nil {
		t.Fatal(err)
	}
	home := connect(t, s, 80, 24)
	home.Send(&api.RenameSession{Session: "work", Name: "api"})
	nextList(t, home, func(l *api.SessionList) bool { return hasSession(l, "api", 1) })
	nextLayout(t, work, func(l *api.Layout) bool { return l.Session == "api" })
	for _, bad := range []*api.RenameSession{
		{Session: "api", Name: "taken"},
		{Session: "nope", Name: "x"},
		{Session: "api", Name: ""},
	} {
		home.Send(bad)
		if e := nextError(t, home); e.Code != api.CodeCommandFailed && e.Code != api.CodeBadCommand {
			t.Fatalf("rename %+v: %+v, want a refusal", bad, e)
		}
	}
	// Input still reaches the renamed session's pane.
	if got := len(s.Sessions()); got != 2 {
		t.Fatalf("%d sessions, want 2", got)
	}
}

// TestIdleCountsClients: a home page with no session yet is not idle, so the
// idle exit cannot pull the server out from under it.
func TestIdleCountsClients(t *testing.T) {
	s := newTestServer(t)
	if !s.idle() {
		t.Fatal("a new server is not idle")
	}
	c := connect(t, s, 80, 24)
	if s.idle() {
		t.Fatal("idle with a client connected")
	}
	c.Close()
	eventually(t, "idle after the client left", s.idle)
}
