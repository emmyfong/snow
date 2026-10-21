package session

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

func TestToAPIKey(t *testing.T) {
	tests := []struct {
		name string
		in   tea.Key
		want api.Key
	}{
		{"letter", tea.Key{Code: 'a', Text: "a"}, api.Key{Code: "a", Text: "a"}},
		{"shifted letter", tea.Key{Code: 'a', Text: "A", Mod: tea.ModShift}, api.Key{Code: "a", Text: "A", Mod: []string{api.ModShift}}},
		{"ctrl+c", tea.Key{Code: 'c', Mod: tea.ModCtrl}, api.Key{Code: "c", Mod: []string{api.ModCtrl}}},
		{"up", tea.Key{Code: tea.KeyUp}, api.Key{Code: api.KeyUp}},
		{"enter", tea.Key{Code: tea.KeyEnter}, api.Key{Code: api.KeyEnter}},
		{"space", tea.Key{Code: tea.KeySpace, Text: " "}, api.Key{Code: api.KeySpace, Text: " "}},
		{"f12 with alt", tea.Key{Code: tea.KeyF12, Mod: tea.ModAlt}, api.Key{Code: api.KeyF12, Mod: []string{api.ModAlt}}},
		{"non-ASCII", tea.Key{Code: 'é', Text: "é"}, api.Key{Code: "é", Text: "é"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toAPIKey(tt.in)
			if !ok || got.Code != tt.want.Code || got.Text != tt.want.Text || !slices.Equal(got.Mod, tt.want.Mod) {
				t.Fatalf("toAPIKey(%+v) = %+v, %v; want %+v", tt.in, got, ok, tt.want)
			}
			if !got.Valid() {
				t.Fatalf("toAPIKey(%+v) = %+v, which api.Key.Valid rejects", tt.in, got)
			}
		})
	}
}

// TestToAPIKeyAltGr: AltGr arrives as Ctrl+Alt with text on Windows. The
// pane must get the text, not a Ctrl+Alt chord the shell sees as ESC (#209).
func TestToAPIKeyAltGr(t *testing.T) {
	got, ok := toAPIKey(tea.Key{Code: '@', Text: "@", Mod: tea.ModCtrl | tea.ModAlt})
	if !ok || got.Text != "@" || len(got.Mod) != 0 {
		t.Fatalf("AltGr @ became %+v, want the text @ with no modifiers", got)
	}
	got, _ = toAPIKey(tea.Key{Code: 'x', Mod: tea.ModCtrl | tea.ModAlt})
	if !slices.Equal(got.Mod, []string{api.ModAlt, api.ModCtrl}) {
		t.Fatalf("Ctrl+Alt+x became %+v, want it to keep its modifiers", got)
	}
}

func TestToAPIKeyDropsUnknown(t *testing.T) {
	if _, ok := toAPIKey(tea.Key{Code: tea.KeyF20}); ok {
		t.Fatal("F20 has no protocol name and should be dropped")
	}
}
