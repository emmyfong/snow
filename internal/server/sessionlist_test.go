package server

import (
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// nextSessionList reads until a SessionList that satisfies ok arrives.
func nextSessionList(t *testing.T, c *apitest.Client, ok func(*api.SessionList) bool) *api.SessionList {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil {
			continue
		}
		if l, is := m.(*api.SessionList); is && ok(l) {
			return l
		}
	}
	t.Fatal("no matching SessionList")
	return nil
}

// TestHomeClientSeesSessions: a client that has not attached, such as the
// home page, gets the session list at once and again whenever it changes.
func TestHomeClientSeesSessions(t *testing.T) {
	s := newTestServer(t)
	home := connect(t, s, 80, 24) // connect checks the first, empty list

	worker := connect(t, s, 80, 24)
	worker.Send(&api.Attach{Session: "work", Create: true})
	l := nextSessionList(t, home, func(l *api.SessionList) bool { return len(l.Sessions) == 1 && l.Sessions[0].Attached == 1 })
	if got := l.Sessions[0]; got.Name != "work" || got.Windows != 1 || got.Attached != 1 || got.LastUsed.IsZero() {
		t.Fatalf("session %+v, want work with 1 window and 1 client", got)
	}

	worker.Close()
	nextSessionList(t, home, func(l *api.SessionList) bool { return len(l.Sessions) == 1 && l.Sessions[0].Attached == 0 })
	if err := s.KillSession("work"); err != nil {
		t.Fatal(err)
	}
	nextSessionList(t, home, func(l *api.SessionList) bool { return len(l.Sessions) == 0 })
}
