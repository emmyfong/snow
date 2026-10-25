package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// usageTurn plays a turn whose reply reports u.
func usageTurn(m *Model, u api.ChatUsage) {
	m.Event(ev(api.ChatEvent{Kind: api.EventTitle, Text: "Fix the rename bug"}))
	oneTurn(m, 1, "Why does rename drop spaces?", "It trims the name before it saves it.")
	m.Event(ev(api.ChatEvent{Kind: api.EventUsage, Usage: &u}))
}

var priced = api.ChatUsage{
	Requested: "anthropic/claude-sonnet-5-5", Reported: "claude-sonnet-5-5",
	Input: 52_000, CacheRead: 9_000, Output: 1_200, Cost: 0.42, Priced: true, Window: 200_000,
}

// TestTopBarGolden: normal, no price, no context window, hidden gauge,
// and a model switch, at 80x24, 120x40, and a narrow pane.
func TestTopBarGolden(t *testing.T) {
	states := map[string]func(m *Model){
		"Normal": func(m *Model) { usageTurn(m, priced) },
		"NoPrice": func(m *Model) {
			u := priced
			u.Cost, u.Priced = 0, false
			usageTurn(m, u)
		},
		"NoWindow": func(m *Model) {
			u := priced
			u.Window = 0
			usageTurn(m, u)
		},
		"Hidden": func(m *Model) {
			m.SetUsage(false)
			usageTurn(m, priced)
		},
		"Switch": func(m *Model) {
			usageTurn(m, priced)
			m.Event(ev(api.ChatEvent{Kind: api.EventModel, Model: "anthropic/claude-opus-5-5", Effort: "high"}))
		},
	}
	for name, state := range states {
		for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 16}} {
			t.Run(fmt.Sprintf("%s%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m, _ := newChat(size[0], size[1])
				state(m)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// TestTopBarText: what each part says.
func TestTopBarText(t *testing.T) {
	tests := []struct {
		name  string
		u     func(*api.ChatUsage)
		usage bool
		want  []string
		not   []string
	}{
		{"priced", func(*api.ChatUsage) {}, true, []string{"Fix the rename bug", "ctx 31%", "$0.42 est.", "anthropic/claude-sonnet-5-5", "mode: ask"}, nil},
		{"no price", func(u *api.ChatUsage) { u.Cost, u.Priced = 0, false }, true, []string{"ctx 31%", "62.2k tok"}, []string{"$"}},
		{"no window", func(u *api.ChatUsage) { u.Window = 0 }, true, []string{"ctx 62.2k tok", "$0.42 est."}, []string{"%"}},
		{"hidden", func(*api.ChatUsage) {}, false, []string{"anthropic/claude-sonnet-5-5"}, []string{"ctx", "$", "tok"}},
		{"hot", func(u *api.ChatUsage) { u.Input = 170_000 }, true, []string{"ctx 90%"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newChat(140, 20)
			m.SetUsage(tt.usage)
			u := priced
			tt.u(&u)
			usageTurn(m, u)
			bar := stripped(m.View()[0])
			for _, w := range tt.want {
				if !strings.Contains(bar, w) {
					t.Errorf("no %q in %q", w, bar)
				}
			}
			for _, w := range tt.not {
				if strings.Contains(bar, w) {
					t.Errorf("%q in %q", w, bar)
				}
			}
		})
	}
}

// TestModelMismatchShowsBoth: a reply from another model than requested
// shows both names in the transcript.
func TestModelMismatchShowsBoth(t *testing.T) {
	m, _ := newChat(120, 20)
	u := priced
	u.Reported = "claude-haiku-5-5"
	usageTurn(m, u)
	s := screen(m)
	if !strings.Contains(s, "Asked for anthropic/claude-sonnet-5-5, but the API reported claude-haiku-5-5.") || !strings.Contains(s, "model") {
		t.Fatalf("screen:\n%s", s)
	}
}

func TestTokens(t *testing.T) {
	for n, want := range map[int]string{0: "0", 950: "950", 1000: "1k", 12_345: "12.3k", 2_000_000: "2M", 1_260_000: "1.3M"} {
		if got := tokens(n); got != want {
			t.Errorf("tokens(%d) = %q, want %q", n, got, want)
		}
	}
	for c, want := range map[float64]string{0: "$0.00", 0.001: "<$0.01", 0.42: "$0.42", 12.345: "$12.35"} {
		if got := dollars(c); got != want {
			t.Errorf("dollars(%v) = %q, want %q", c, got, want)
		}
	}
}
