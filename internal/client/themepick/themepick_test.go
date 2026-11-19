package themepick

import (
	"testing"

	"github.com/emmyfong/snow/internal/client/theme"
)

func TestThemePickState(t *testing.T) {
	themes := theme.Builtin()
	frost, _ := theme.ByName("Frost")
	campbell, _ := theme.ByName("Campbell")
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "move clamps at first and last", run: func(t *testing.T) {
			m := New(themes[:2], frost)
			m.Move(-1)
			if got := m.Selected(); got != 0 {
				t.Fatalf("selected = %d, want 0", got)
			}
			if got := m.Move(99); got.Name != "Campbell" || m.Selected() != 1 {
				t.Fatalf("Move(99) = %q at %d", got.Name, m.Selected())
			}
			m.Move(1)
			if got := m.Selected(); got != 1 {
				t.Fatalf("selected = %d, want 1", got)
			}
		}},
		{name: "restore and keep", run: func(t *testing.T) {
			m := New(themes[:2], frost)
			m.Move(1)
			if got := m.Restore(); got.Name != frost.Name {
				t.Fatalf("Restore = %q, want %q", got.Name, frost.Name)
			}
			if got := m.Keep(); got.Name != campbell.Name {
				t.Fatalf("Keep = %q, want %q", got.Name, campbell.Name)
			}
		}},
		{name: "items mark original theme", run: func(t *testing.T) {
			m := New(themes[:2], frost)
			m.Move(1)
			items := m.Items()
			if len(items) != 2 || len(items[0].Cols) < 2 || items[0].Cols[1] != "in use" || items[1].Cols[1] != "" {
				t.Fatalf("Items = %#v, want only Frost marked in use", items)
			}
		}},
		{name: "empty list shows before", run: func(t *testing.T) {
			m := New(nil, frost)
			if got := m.Shown(); got.Name != frost.Name {
				t.Fatalf("Shown = %q, want %q", got.Name, frost.Name)
			}
			if got := m.Move(1); got.Name != frost.Name {
				t.Fatalf("Move on empty list = %q, want %q", got.Name, frost.Name)
			}
			if got := m.Items(); len(got) != 0 {
				t.Fatalf("Items = %#v, want empty", got)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
