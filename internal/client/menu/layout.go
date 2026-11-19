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
	{"Help", "/help"},
	{"Leave Snow running", "detach"},
}

// LayoutItems returns the rows in the window menu. connect is true when the
// focused pane can open a login or runtime picker.
func LayoutItems(t *keys.Table, connect bool) []Item {
	rows := layoutRows
	if !connect {
		rows = make([]rowSpec, 0, len(layoutRows)-1)
		for _, row := range layoutRows {
			if row.action != "connect" {
				rows = append(rows, row)
			}
		}
	}
	return buildItems(t, rows)
}
