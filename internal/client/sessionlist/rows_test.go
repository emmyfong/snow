package sessionlist

import (
	"testing"
	"time"

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
	items := Items([]api.SessionInfo{info("api", 5*time.Minute, 1)}, base)
	if len(items) != 1 {
		t.Fatalf("%d items, want 1", len(items))
	}
	if c := items[0].Cols; len(c) != 3 || c[0] != "api" || c[1] != "running, 1 client" || c[2] != "5m ago" {
		t.Fatalf("cols %q", c)
	}
}
