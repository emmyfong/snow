package menu

import (
	"testing"

	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/config"
)

func TestLayoutMenuRows(t *testing.T) {
	for _, tt := range []struct {
		name    string
		connect bool
		want    []Item
	}{
		{name: "connect available", connect: true, want: []Item{
			{"New chat", "", "new-chat"}, {"New terminal", "Ctrl+b c", "new-window"},
			{"Split", "Ctrl+b %", "split-right"}, {"Sessions", "Ctrl+b s", "sessions"},
			{"Agents", "Ctrl+b w", "choose-agent"}, {"Connect", "", "connect"},
			{"Theme", "", "/theme"}, {"Help", "", "/help"},
			{"Leave Snow running", "Ctrl+b d", "detach"},
		}},
		{name: "connect unavailable", want: []Item{
			{"New chat", "", "new-chat"}, {"New terminal", "Ctrl+b c", "new-window"},
			{"Split", "Ctrl+b %", "split-right"}, {"Sessions", "Ctrl+b s", "sessions"},
			{"Agents", "Ctrl+b w", "choose-agent"}, {"Theme", "", "/theme"},
			{"Help", "", "/help"},
			{"Leave Snow running", "Ctrl+b d", "detach"},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items := LayoutItems(keys.Default(), tt.connect)
			if len(items) != len(tt.want) {
				t.Fatalf("got %d rows: %+v", len(items), items)
			}
			for i := range tt.want {
				if items[i] != tt.want[i] {
					t.Errorf("row %d = %+v, want %+v", i, items[i], tt.want[i])
				}
			}
		})
	}
}

func TestLayoutMenuActionsAreUnique(t *testing.T) {
	for _, connect := range []bool{false, true} {
		t.Run(map[bool]string{false: "connect unavailable", true: "connect available"}[connect], func(t *testing.T) {
			seen := make(map[string]string)
			for _, item := range LayoutItems(keys.Default(), connect) {
				if previous, ok := seen[item.Action]; ok {
					t.Errorf("%q and %q share action %q", previous, item.Label, item.Action)
				}
				seen[item.Action] = item.Label
			}
		})
	}
}

func TestLayoutMenuShortcutsFromKeys(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	cfg.Bind = map[string]string{"x": "split-right", "%": config.UnbindAction}
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range LayoutItems(tab, true) {
		if item.Action == "split-right" && item.Shortcut != "Ctrl+a x" {
			t.Fatalf("Split shortcut = %q", item.Shortcut)
		}
	}
}
