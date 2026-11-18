package menu

import (
	"testing"

	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/config"
)

func TestLayoutMenuRows(t *testing.T) {
	items := LayoutItems(keys.Default())
	want := []Item{
		{"New chat", "", "new-chat"}, {"New terminal", "Ctrl+b c", "new-window"},
		{"Split", "Ctrl+b %", "split-right"}, {"Sessions", "Ctrl+b s", "sessions"},
		{"Agents", "Ctrl+b w", "choose-agent"}, {"Connect", "", "connect"},
		{"Theme", "", "/theme"}, {"Keyboard shortcuts", "", "/help"},
		{"Help", "", "/help"}, {"Leave Snow running", "Ctrl+b d", "detach"},
	}
	if len(items) != len(want) {
		t.Fatalf("got %d rows: %+v", len(items), items)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, items[i], want[i])
		}
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
	for _, item := range LayoutItems(tab) {
		if item.Action == "split-right" && item.Shortcut != "Ctrl+a x" {
			t.Fatalf("Split shortcut = %q", item.Shortcut)
		}
	}
}
