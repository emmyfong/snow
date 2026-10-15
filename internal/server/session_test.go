package server

import (
	"errors"
	"slices"
	"testing"
)

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
