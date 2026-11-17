package menu

import "github.com/emmyfong/snow/internal/client/keys"

var layoutRows = []rowSpec{
	{"New chat", "new-chat"},
	{"New terminal", "new-window"},
	{"Split", "split-right"},
	{"Sessions", "sessions"},
	{"Agents", "choose-agent"},
	{"Connect", "connect"},
	{"Theme", "/theme"},
	{"Keyboard shortcuts", "/help"},
	{"Help", "/help"},
	{"Leave Snow running", "detach"},
}

// LayoutItems returns the rows in the window menu.
func LayoutItems(t *keys.Table) []Item { return buildItems(t, layoutRows) }
