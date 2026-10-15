package layout

import (
	"errors"
	"testing"
)

func TestResizeCells(t *testing.T) {
	tests := []struct {
		name       string
		tree       *Tree
		pane       PaneID
		dir        Direction
		cells      int
		cols, rows int
		want       map[PaneID]Rect
	}{
		{
			name: "grow right by 5 cells",
			tree: &Tree{Root: h(half, leaf(1), leaf(2))}, pane: 1, dir: Right, cells: 5, cols: 21, rows: 4,
			want: map[PaneID]Rect{1: {0, 0, 15, 4}, 2: {16, 0, 5, 4}},
		},
		{
			// The cells are measured in the nested split, not the window.
			name: "nested split",
			tree: &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}, pane: 2, dir: Down, cells: 3, cols: 21, rows: 11,
			want: map[PaneID]Rect{1: {0, 0, 10, 11}, 2: {11, 0, 10, 8}, 3: {11, 9, 10, 2}},
		},
		{
			name: "shrink by moving the left edge in",
			tree: &Tree{Root: h(half, leaf(1), leaf(2))}, pane: 2, dir: Left, cells: -4, cols: 21, rows: 4,
			want: map[PaneID]Rect{1: {0, 0, 14, 4}, 2: {15, 0, 6, 4}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.tree.ResizeCells(tt.pane, tt.dir, tt.cells, tt.cols, tt.rows); err != nil {
				t.Fatal(err)
			}
			g := Rects(tt.tree, tt.cols, tt.rows)
			for id, want := range tt.want {
				if g.Panes[id] != want {
					t.Errorf("pane %d = %v, want %v; all %v", id, g.Panes[id], want, g.Panes)
				}
			}
		})
	}
}

func TestResizeCellsNoEdge(t *testing.T) {
	tr := &Tree{Root: h(half, leaf(1), leaf(2))}
	if err := tr.ResizeCells(1, Left, 3, 20, 5); !errors.Is(err, ErrNoEdge) {
		t.Fatalf("ResizeCells at the window border = %v, want ErrNoEdge", err)
	}
}
