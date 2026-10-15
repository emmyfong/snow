package layout

import (
	"reflect"
	"testing"
)

func TestRects(t *testing.T) {
	tests := []struct {
		name       string
		tree       *Tree
		cols, rows int
		want       map[PaneID]Rect
	}{
		{
			name: "one pane fills the window",
			tree: New(1), cols: 80, rows: 24,
			want: map[PaneID]Rect{1: {0, 0, 80, 24}},
		},
		{
			// The example in #19. Edges round half up, as in tmux, so the
			// first child of an odd split gets the extra cell.
			name: "issue example",
			tree: &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}, cols: 120, rows: 40,
			want: map[PaneID]Rect{1: {0, 0, 60, 40}, 2: {61, 0, 59, 20}, 3: {61, 21, 59, 19}},
		},
		{
			name: "odd rows",
			tree: &Tree{Root: v(half, leaf(1), leaf(2))}, cols: 81, rows: 25,
			want: map[PaneID]Rect{1: {0, 0, 81, 12}, 2: {0, 13, 81, 12}},
		},
		{
			name: "three children",
			tree: &Tree{Root: h([]float64{0.25, 0.25, 0.5}, leaf(1), leaf(2), leaf(3))}, cols: 10, rows: 5,
			want: map[PaneID]Rect{1: {0, 0, 2, 5}, 2: {3, 0, 2, 5}, 3: {6, 0, 4, 5}},
		},
		{
			name: "thirds",
			tree: &Tree{Root: h([]float64{1.0 / 3, 2.0 / 3}, leaf(1), leaf(2))}, cols: 31, rows: 1,
			want: map[PaneID]Rect{1: {0, 0, 10, 1}, 2: {11, 0, 20, 1}},
		},
		{
			name: "grid at odd size",
			tree: &Tree{Root: v(half, h(half, leaf(1), leaf(2)), h([]float64{0.25, 0.75}, leaf(3), leaf(4)))}, cols: 81, rows: 21,
			want: map[PaneID]Rect{1: {0, 0, 40, 10}, 2: {41, 0, 40, 10}, 3: {0, 11, 20, 10}, 4: {21, 11, 60, 10}},
		},
		{
			name: "zoomed pane fills the window",
			tree: &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3))), Zoomed: 3}, cols: 120, rows: 40,
			want: map[PaneID]Rect{3: {0, 0, 120, 40}},
		},
		{
			name: "too narrow for both panes",
			tree: &Tree{Root: h(half, leaf(1), leaf(2))}, cols: 2, rows: 1,
			want: map[PaneID]Rect{1: {0, 0, 1, 1}, 2: {2, 0, 0, 1}},
		},
		{
			name: "too narrow for the borders",
			tree: &Tree{Root: h(half, leaf(1), h(half, leaf(2), leaf(3)))}, cols: 1, rows: 1,
			want: map[PaneID]Rect{1: {0, 0, 0, 1}, 2: {1, 0, 0, 1}, 3: {1, 0, 0, 1}},
		},
		{
			name: "empty window",
			tree: &Tree{Root: v(half, leaf(1), leaf(2))}, cols: 0, rows: 0,
			want: map[PaneID]Rect{1: {0, 0, 0, 0}, 2: {0, 0, 0, 0}},
		},
		{
			name: "negative size counts as empty",
			tree: New(1), cols: -5, rows: -1,
			want: map[PaneID]Rect{1: {0, 0, 0, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := Rects(tt.tree, tt.cols, tt.rows)
			if !reflect.DeepEqual(g.Panes, tt.want) {
				t.Fatalf("Panes = %v, want %v", g.Panes, tt.want)
			}
			checkCover(t, g, max(tt.cols, 0), max(tt.rows, 0))
		})
	}
}

// checkCover fails unless the panes and borders of g lie inside the window,
// do not overlap, and together cover every cell.
func checkCover(t *testing.T, g Geometry, cols, rows int) {
	t.Helper()
	var all []Rect
	for _, r := range g.Panes {
		all = append(all, r)
	}
	all = append(all, g.Borders...)
	area := 0
	for i, a := range all {
		if a.W < 0 || a.H < 0 || a.X < 0 || a.Y < 0 || a.X+a.W > cols || a.Y+a.H > rows {
			t.Fatalf("rect %v is outside the %dx%d window", a, cols, rows)
		}
		area += a.W * a.H
		for _, b := range all[i+1:] {
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Fatalf("rects %v and %v overlap", a, b)
			}
		}
	}
	if area != cols*rows {
		t.Fatalf("rects cover %d cells, want %d", area, cols*rows)
	}
}

func TestBorders(t *testing.T) {
	g := Rects(&Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}, 120, 40)
	want := []Rect{{60, 0, 1, 40}, {61, 20, 59, 1}}
	if !reflect.DeepEqual(g.Borders, want) {
		t.Fatalf("Borders = %v, want %v", g.Borders, want)
	}
}
