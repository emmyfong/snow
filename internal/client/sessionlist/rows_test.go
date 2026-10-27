package sessionlist

import (
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var base = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func info(name string, ago time.Duration, attached int) api.SessionInfo {
	return api.SessionInfo{Name: name, State: api.StateRunning, Attached: attached, LastUsed: base.Add(-ago)}
}

func TestAgo(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{30 * time.Hour, "yesterday"},
		{100 * time.Hour, "4 days ago"},
	}
	for _, tt := range tests {
		if got := Ago(tt.d); got != tt.want {
			t.Errorf("Ago(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestState(t *testing.T) {
	tests := []struct {
		attached int
		want     string
	}{
		{0, "running"},
		{1, "running, 1 client"},
		{3, "running, 3 clients"},
	}
	for _, tt := range tests {
		if got := State(info("a", 0, tt.attached)); got != tt.want {
			t.Errorf("State with %d clients = %q, want %q", tt.attached, got, tt.want)
		}
	}
}

// TestSortNewestFirst: the most recently used session comes first; a tie
// keeps name order; the input is not changed.
func TestSortNewestFirst(t *testing.T) {
	in := []api.SessionInfo{info("old", time.Hour, 0), info("b", time.Minute, 0), info("a", time.Minute, 0)}
	got := Sort(in)
	if got[0].Name != "a" || got[1].Name != "b" || got[2].Name != "old" {
		t.Fatalf("sorted %v, want a b old", []string{got[0].Name, got[1].Name, got[2].Name})
	}
	if in[0].Name != "old" {
		t.Fatal("Sort changed its input")
	}
}

func TestItems(t *testing.T) {
	items := Items([]api.SessionInfo{info("api", 5*time.Minute, 1)}, base, theme.Default().Glyphs)
	if len(items) != 1 {
		t.Fatalf("%d items, want 1", len(items))
	}
	if c := items[0].Cols; len(c) != 3 || c[0] != "api" || c[1] != "running, 1 client" || c[2] != "5m ago" {
		t.Fatalf("cols %q", c)
	}
}

// TestNeedsYouAlert: a session where agents wait says so in words, with
// the count when more than one waits; the mark is the theme's glyph.
func TestNeedsYouAlert(t *testing.T) {
	g := theme.Default().Glyphs
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "● needs you"},
		{3, "● needs you (3)"},
	}
	for _, tt := range tests {
		s := info("api", time.Minute, 0)
		s.NeedsYou = tt.n
		if got := Items([]api.SessionInfo{s}, base, g)[0].Alert; got != tt.want {
			t.Errorf("%d waiting: alert %q, want %q", tt.n, got, tt.want)
		}
	}
	g.AgentNeedsYou = "!"
	s := info("api", time.Minute, 0)
	s.NeedsYou = 1
	if got := Items([]api.SessionInfo{s}, base, g)[0].Alert; got != "! needs you" {
		t.Errorf("alert %q ignores the theme's glyph", got)
	}
}
