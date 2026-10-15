package mouse

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// threePanes is pane 1 on the left and panes 2 and 3 stacked on the right,
// in a window of 80 by 23 cells: borders at column 40 and, on the right, row 12.
func threePanes() []api.PaneInfo {
	return []api.PaneInfo{
		{ID: 1, Rect: api.Rect{X: 0, Y: 0, W: 40, H: 23}},
		{ID: 2, Rect: api.Rect{X: 41, Y: 0, W: 39, H: 12}},
		{ID: 3, Rect: api.Rect{X: 41, Y: 13, W: 39, H: 10}},
	}
}

var tabs = []TabSpan{{Index: 0, X: 7, W: 5}, {Index: 1, X: 13, W: 7}}

func TestHitTest(t *testing.T) {
	const cols, rows = 80, 24
	tests := []struct {
		name string
		x, y int
		want Hit
	}{
		{"pane 1", 5, 5, Hit{Kind: Pane, Pane: 1, X: 5, Y: 5}},
		{"pane 1 corner", 0, 0, Hit{Kind: Pane, Pane: 1}},
		{"pane 1 last cell", 39, 22, Hit{Kind: Pane, Pane: 1, X: 39, Y: 22}},
		{"pane 2 is relative to its corner", 45, 3, Hit{Kind: Pane, Pane: 2, X: 4, Y: 3}},
		{"pane 3 is relative to its corner", 45, 20, Hit{Kind: Pane, Pane: 3, X: 4, Y: 7}},
		{"vertical border beside pane 2", 40, 5, Hit{Kind: Border, Axis: Vertical, Before: 1, After: 2}},
		{"vertical border beside pane 3", 40, 20, Hit{Kind: Border, Axis: Vertical, Before: 1, After: 3}},
		{"horizontal border", 60, 12, Hit{Kind: Border, Axis: Horizontal, Before: 2, After: 3}},
		{"junction picks the vertical border", 40, 12, Hit{Kind: Border, Axis: Vertical, Before: 1, After: 2}},
		{"tab 0", 7, 23, Hit{Kind: Tab, Window: 0}},
		{"tab 0 last cell", 11, 23, Hit{Kind: Tab, Window: 0}},
		{"gap between tabs", 12, 23, Hit{Kind: Bar}},
		{"tab 1", 19, 23, Hit{Kind: Tab, Window: 1}},
		{"status bar elsewhere", 70, 23, Hit{Kind: Bar}},
		{"session name", 2, 23, Hit{Kind: Bar}},
		{"outside the screen", 80, 5, Hit{Kind: None}},
		{"below the screen", 5, 24, Hit{Kind: None}},
		{"negative", -1, 5, Hit{Kind: None}},
	}
	for _, tt := range tests {
		if got := HitTest(threePanes(), tabs, tt.x, tt.y, cols, rows); got != tt.want {
			t.Errorf("%s: HitTest(%d,%d) = %+v, want %+v", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
}

// A client larger than the session shows blank space beyond the panes. It
// is not a border.
func TestHitTestBlankAreaIsNothing(t *testing.T) {
	if got := HitTest(threePanes(), nil, 90, 5, 100, 30); got.Kind != None {
		t.Errorf("blank space right of the panes = %+v, want None", got)
	}
	if got := HitTest(threePanes(), nil, 5, 25, 100, 30); got.Kind != None {
		t.Errorf("blank space below the panes = %+v, want None", got)
	}
	if got := HitTest(threePanes(), nil, 5, 29, 100, 30); got.Kind != Bar {
		t.Errorf("the status bar of a big screen = %+v, want Bar", got)
	}
}

func TestHitTestWithoutPanes(t *testing.T) {
	if got := HitTest(nil, nil, 3, 3, 80, 24); got.Kind != None {
		t.Errorf("no panes: %+v, want None", got)
	}
	if got := HitTest(nil, nil, 3, 23, 80, 24); got.Kind != Bar {
		t.Errorf("no panes, status row: %+v, want Bar", got)
	}
	if got := HitTest(threePanes(), nil, 3, 3, 0, 0); got.Kind != None {
		t.Errorf("zero screen: %+v, want None", got)
	}
}

// Nested splits: a vertical border whose two sides are cut into different
// rows still names the panes beside the clicked row.
func TestHitTestNestedBorders(t *testing.T) {
	panes := []api.PaneInfo{
		{ID: 1, Rect: api.Rect{X: 0, Y: 0, W: 20, H: 10}},
		{ID: 2, Rect: api.Rect{X: 0, Y: 11, W: 20, H: 11}},
		{ID: 3, Rect: api.Rect{X: 21, Y: 0, W: 59, H: 22}},
	}
	tests := []struct {
		x, y   int
		before int
		after  int
	}{{20, 3, 1, 3}, {20, 15, 2, 3}, {20, 10, 1, 3}}
	for _, tt := range tests {
		got := HitTest(panes, nil, tt.x, tt.y, 80, 23)
		if got.Kind != Border || got.Axis != Vertical || got.Before != tt.before || got.After != tt.after {
			t.Errorf("(%d,%d) = %+v, want vertical border %d|%d", tt.x, tt.y, got, tt.before, tt.after)
		}
	}
	// The T-junction on the left half belongs to the horizontal border
	// between panes 1 and 2 when the click is left of the vertical one.
	got := HitTest(panes, nil, 5, 10, 80, 23)
	if got.Axis != Horizontal || got.Before != 1 || got.After != 2 {
		t.Errorf("(5,10) = %+v, want the horizontal border 1/2", got)
	}
}
