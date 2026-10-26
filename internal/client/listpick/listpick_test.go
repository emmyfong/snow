package listpick

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

func rows(n int) [][]string {
	out := make([][]string, n)
	for i := range out {
		out[i] = []string{string(rune('a' + i)), "row"}
	}
	return out
}

func press(m *Model, code rune) picker.Result { return m.Key(tea.Key{Code: code}) }

func TestKeys(t *testing.T) {
	tests := []struct {
		name string
		n    int
		keys []tea.Key
		want int
		res  picker.Result
	}{
		{"down", 3, []tea.Key{{Code: tea.KeyDown}}, 1, picker.Open},
		{"up wraps", 3, []tea.Key{{Code: tea.KeyUp}}, 2, picker.Open},
		{"j and k", 3, []tea.Key{{Text: "j", Code: 'j'}, {Text: "j", Code: 'j'}, {Text: "k", Code: 'k'}}, 1, picker.Open},
		{"end then home", 9, []tea.Key{{Code: tea.KeyEnd}, {Code: tea.KeyHome}}, 0, picker.Open},
		{"page down stops at the end", 7, []tea.Key{{Code: tea.KeyPgDown}, {Code: tea.KeyPgDown}}, 6, picker.Open},
		{"enter accepts", 3, []tea.Key{{Code: tea.KeyDown}, {Code: tea.KeyEnter}}, 1, picker.Accepted},
		{"escape cancels", 3, []tea.Key{{Code: tea.KeyEscape}}, 0, picker.Cancelled},
		{"enter on nothing closes", 0, []tea.Key{{Code: tea.KeyEnter}}, -1, picker.Cancelled},
		{"keys on nothing do not crash", 0, []tea.Key{{Code: tea.KeyDown}, {Code: tea.KeyEnd}}, -1, picker.Open},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New("T", nil)
			m.SetRows(rows(tt.n))
			var res picker.Result
			for _, k := range tt.keys {
				res = m.Key(k)
			}
			if res != tt.res || m.Selected() != tt.want {
				t.Fatalf("result %v, selected %d; want %v, %d", res, m.Selected(), tt.res, tt.want)
			}
		})
	}
}

func TestSetRowsKeepsSelectionInRange(t *testing.T) {
	m := New("T", nil)
	m.SetRows(rows(5))
	m.Select(4)
	m.SetRows(rows(2))
	if m.Selected() != 1 {
		t.Fatalf("selected %d, want 1", m.Selected())
	}
}

func TestClick(t *testing.T) {
	th := theme.Default()
	m := New("T", []ui.Hint{{Key: "Esc", Desc: "close"}})
	m.SetRows(rows(3))
	box, x, y := m.View(th, 80, 24)
	if box == "" || !strings.Contains(box, "row") {
		t.Fatalf("no popup:\n%s", box)
	}
	if got := m.Click(th, 80, 24, x+4, y+2); got != picker.Accepted || m.Selected() != 1 {
		t.Fatalf("click on the second row: %v, selected %d", got, m.Selected())
	}
	if got := m.Click(th, 80, 24, x+4, y); got != picker.Open {
		t.Fatalf("click on the frame: %v", got)
	}
	if got := m.Click(th, 80, 24, 0, 0); got != picker.Cancelled {
		t.Fatalf("click outside: %v", got)
	}
}

func TestSmallWindowDrawsNothing(t *testing.T) {
	m := New("T", nil)
	m.SetRows(rows(3))
	if box, _, _ := m.View(theme.Default(), 20, 24); box != "" {
		t.Fatal("drew in a window that is too narrow")
	}
	if got := press(m, tea.KeyDown); got != picker.Open || m.Selected() != 1 {
		t.Fatal("keys stopped answering when the popup could not draw")
	}
}
