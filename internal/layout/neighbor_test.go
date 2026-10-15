package layout

import (
	"errors"
	"testing"
)

func TestNeighbor(t *testing.T) {
	// 120x40: 1 = [0,0 59x40], 2 = [60,0 60x19], 3 = [60,20 60x20].
	issue := &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}
	// 81x21: top row 1 = [0,0 40x10], 2 = [41,0 40x10];
	// bottom row 3 = [0,11 20x10], 4 = [21,11 60x10].
	grid := &Tree{Root: v(half, h(half, leaf(1), leaf(2)), h([]float64{0.25, 0.75}, leaf(3), leaf(4)))}
	zoomed := &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3))), Zoomed: 1}

	tests := []struct {
		name       string
		tree       *Tree
		cols, rows int
		from       PaneID
		dir        Direction
		want       PaneID
		wantErr    error
	}{
		{"right picks the topmost overlapping pane", issue, 120, 40, 1, Right, 2, nil},
		{"left", issue, 120, 40, 3, Left, 1, nil},
		{"down", issue, 120, 40, 2, Down, 3, nil},
		{"up", issue, 120, 40, 3, Up, 2, nil},
		{"left at the window edge", issue, 120, 40, 1, Left, 0, ErrNoNeighbor},
		{"right at the window edge", issue, 120, 40, 2, Right, 0, ErrNoNeighbor},
		{"up at the window edge", issue, 120, 40, 2, Up, 0, ErrNoNeighbor},
		{"up picks the leftmost overlapping pane", grid, 81, 21, 4, Up, 1, nil},
		{"down across misaligned columns", grid, 81, 21, 2, Down, 4, nil},
		{"down picks the leftmost overlapping pane", grid, 81, 21, 1, Down, 3, nil},
		{"right in the bottom row", grid, 81, 21, 3, Right, 4, nil},
		{"zoom does not hide neighbors", zoomed, 120, 40, 1, Right, 2, nil},
		{"unknown pane", issue, 120, 40, 9, Right, 0, ErrNotFound},
		{"invalid direction", issue, 120, 40, 1, Direction(0), 0, ErrInvalidDirection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Neighbor(tt.tree, tt.from, tt.dir, tt.cols, tt.rows)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Neighbor = %d, want %d", got, tt.want)
			}
		})
	}
}
