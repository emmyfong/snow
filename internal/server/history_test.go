package server

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// printNumbers makes the pane's shell print 1 to n, one per line.
func printNumbers(c *apitest.Client, pane, n int) {
	cmd := "seq 1 " + strconv.Itoa(n) + "\r"
	if runtime.GOOS == "windows" {
		cmd = "for /l %i in (1,1," + strconv.Itoa(n) + ") do @echo %i\r"
	}
	c.Send(&api.Input{Pane: pane, Paste: cmd})
	c.WaitScreen("\n" + strconv.Itoa(n) + "\n")
}

// nextOf reads until a message of type T arrives. An Error fails the test.
func nextOf[T api.Message](t *testing.T, c *apitest.Client) T {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if e, ok := m.(*api.Error); ok {
			t.Fatalf("server error: %v", e)
		}
		if got, ok := m.(T); ok {
			return got
		}
	}
	var zero T
	t.Fatalf("no %T", zero)
	return zero
}

func TestScrollbackPages(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 80, 24)
	printNumbers(c, pane, 100)

	c.Send(&api.ScrollbackRequest{Pane: pane, From: 0, Count: 10})
	first := nextOf[*api.Scrollback](t, c)
	if first.Pane != pane || first.From != 0 || len(first.Lines) != 10 {
		t.Fatalf("first page: pane %d from %d with %d lines, want pane %d from 0 with 10", first.Pane, first.From, len(first.Lines), pane)
	}
	if first.Total < 100 {
		t.Fatalf("total %d, want at least the 100 printed lines", first.Total)
	}

	c.Send(&api.ScrollbackRequest{Pane: pane, From: first.Total - 3, Count: 50})
	last := nextOf[*api.Scrollback](t, c)
	if len(last.Lines) != 3 || last.From != first.Total-3 {
		t.Fatalf("last page: from %d with %d lines, want from %d with 3 (clipped at the end)", last.From, len(last.Lines), first.Total-3)
	}

	c.Send(&api.ScrollbackRequest{Pane: pane, From: first.Total + 5, Count: 5})
	if past := nextOf[*api.Scrollback](t, c); len(past.Lines) != 0 {
		t.Fatalf("a page past the end has %d lines, want none", len(past.Lines))
	}
}

func TestScrollbackRefusals(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 80, 24)
	tests := []struct {
		name string
		req  *api.ScrollbackRequest
		code string
	}{
		{"unknown pane", &api.ScrollbackRequest{Pane: pane + 50, Count: 5}, api.CodeCommandFailed},
		{"bad range", &api.ScrollbackRequest{Pane: pane, Count: 0}, api.CodeBadCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Send(tt.req)
			for {
				m, err := c.Next(5 * time.Second)
				if err != nil {
					t.Fatalf("no answer: %v", err)
				}
				if e, ok := m.(*api.Error); ok {
					if e.Code != tt.code {
						t.Fatalf("code %s, want %s", e.Code, tt.code)
					}
					return
				}
				if _, ok := m.(*api.Scrollback); ok {
					t.Fatal("got a Scrollback, want an Error")
				}
			}
		})
	}
}

func TestSearchAnswersWithPositions(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 80, 24)
	printNumbers(c, pane, 100)

	c.Send(&api.SearchRequest{Pane: pane, Query: "99"})
	res := nextOf[*api.SearchResult](t, c)
	if res.Pane != pane || res.Query != "99" || res.Truncated {
		t.Fatalf("result %+v, want pane %d, query 99, not truncated", res, pane)
	}
	if len(res.Matches) == 0 {
		t.Fatal("no match for 99 in the numbers 1 to 100")
	}
	// The matches' lines hold what the search said: ask for the first.
	m := res.Matches[0]
	c.Send(&api.ScrollbackRequest{Pane: pane, From: m.Line, Count: 1})
	got := nextOf[*api.Scrollback](t, c)
	if len(got.Lines) != 1 || !strings.Contains(got.Lines[0], "99") || m.Len != 2 {
		t.Fatalf("line %d is %q (match %+v), want a line with 99", m.Line, got.Lines, m)
	}
}

func TestSearchRefusals(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 80, 24)
	for _, tt := range []struct {
		name string
		req  *api.SearchRequest
		code string
	}{
		{"unknown pane", &api.SearchRequest{Pane: pane + 50, Query: "x"}, api.CodeCommandFailed},
		{"empty query", &api.SearchRequest{Pane: pane}, api.CodeBadCommand},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c.Send(tt.req)
			for {
				m, err := c.Next(5 * time.Second)
				if err != nil {
					t.Fatalf("no answer: %v", err)
				}
				if e, ok := m.(*api.Error); ok {
					if e.Code != tt.code {
						t.Fatalf("code %s, want %s", e.Code, tt.code)
					}
					return
				}
			}
		})
	}
}
