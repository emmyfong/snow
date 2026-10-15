package keys

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/config"
)

func entryFor(t *testing.T, es []Entry, desc string) Entry {
	t.Helper()
	for _, e := range es {
		if e.Desc == desc {
			return e
		}
	}
	t.Fatalf("no entry %q in %+v", desc, es)
	return Entry{}
}

func TestEntriesGroupRelatedKeys(t *testing.T) {
	es := Default().Entries()
	tests := map[string]string{
		"Split right":      "%",
		"Select window":    "0-9",
		"Focus pane":       "Arrows",
		"Resize pane by 1": "Ctrl+Arrows",
		"Resize pane by 5": "Alt+Arrows",
		"Detach":           "d",
	}
	for desc, keys := range tests {
		if got := entryFor(t, es, desc).Keys; got != keys {
			t.Errorf("%s keys = %q, want %q", desc, got, keys)
		}
	}
	if len(es) > 20 {
		t.Errorf("%d entries; the popup must fit a 24-row screen", len(es))
	}
}

func TestEntriesFollowRebinding(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"|": "split-right", "%": config.UnbindAction, "C-x": "zoom"}
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	es := tab.Entries()
	if got := entryFor(t, es, "Split right").Keys; got != "|" {
		t.Errorf("Split right keys = %q, want |", got)
	}
	if got := entryFor(t, es, "Zoom").Keys; got != "z/Ctrl+x" {
		t.Errorf("Zoom keys = %q, want z/Ctrl+x", got)
	}
}

func TestEntriesSkipUnboundActions(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"d": config.UnbindAction}
	tab, _ := New(cfg)
	for _, e := range tab.Entries() {
		if strings.Contains(e.Desc, "Detach") {
			t.Errorf("unbound action listed: %+v", e)
		}
	}
}

func TestPartialGroupListsItsKeys(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"Left": config.UnbindAction, "Right": config.UnbindAction}
	tab, _ := New(cfg)
	if got := entryFor(t, tab.Entries(), "Focus pane").Keys; got != "Up/Down" {
		t.Errorf("keys = %q, want Up/Down", got)
	}
}
