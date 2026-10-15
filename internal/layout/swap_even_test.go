package layout

import (
	"errors"
	"math"
	"testing"
)

func TestSwap(t *testing.T) {
	tr := &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3))), Zoomed: 2}
	if err := tr.Swap(1, 3); err != nil {
		t.Fatal(err)
	}
	want := &Tree{Root: h(half, leaf(3), v(half, leaf(2), leaf(1)))}
	if dump(t, tr) != dump(t, want) {
		t.Fatalf("after swap: %s, want %s", dump(t, tr), dump(t, want))
	}
	if err := tr.Swap(1, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Swap with a missing pane = %v, want ErrNotFound", err)
	}
	if err := tr.Swap(2, 2); err != nil {
		t.Fatalf("Swap of a pane with itself = %v, want nil", err)
	}
}

func TestEvenOut(t *testing.T) {
	quarter := []float64{0.5, 0.25, 0.25}
	tests := []struct {
		name  string
		start *Tree
		pane  PaneID
		// widths are the visible shares of the panes in the evened row.
		want map[PaneID]float64
	}{
		{
			name:  "flat row",
			start: &Tree{Root: h(quarter, leaf(1), leaf(2), leaf(3))},
			pane:  2,
			want:  map[PaneID]float64{1: 1.0 / 3, 2: 1.0 / 3, 3: 1.0 / 3},
		},
		{
			// Repeated splits nest, but evening out the row treats the chain
			// of same-direction splits as one row.
			name:  "nested row",
			start: &Tree{Root: h(half, leaf(1), h(half, leaf(2), leaf(3)))},
			pane:  3,
			want:  map[PaneID]float64{1: 1.0 / 3, 2: 1.0 / 3, 3: 1.0 / 3},
		},
		{
			// A column inside the row counts as one cell of the row.
			name:  "column inside a row",
			start: &Tree{Root: h([]float64{0.8, 0.2}, leaf(1), v(half, leaf(2), leaf(3)))},
			pane:  1,
			want:  map[PaneID]float64{1: 0.5, 2: 0.5, 3: 0.5},
		},
		{
			name:  "evens the column the pane is in",
			start: &Tree{Root: h(half, leaf(1), v([]float64{0.9, 0.1}, leaf(2), leaf(3)))},
			pane:  3,
			want:  map[PaneID]float64{2: 0.5, 3: 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.start.Even(tt.pane); err != nil {
				t.Fatal(err)
			}
			if err := tt.start.validate(); err != nil {
				t.Fatalf("tree invalid after Even: %v", err)
			}
			for id, want := range tt.want {
				if got := share(t, tt.start, id); math.Abs(got-want) > 1e-9 {
					t.Errorf("pane %d share = %v, want %v; tree %s", id, got, want, dump(t, tt.start))
				}
			}
		})
	}
}

func TestEvenOutSinglePane(t *testing.T) {
	tr := New(1)
	if err := tr.Even(1); err != nil {
		t.Fatalf("Even on a lone pane = %v, want nil", err)
	}
	if err := tr.Even(5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Even on a missing pane = %v, want ErrNotFound", err)
	}
}

// share returns pane id's fraction of the window along the axis of the split
// that directly holds it, multiplied up through ancestors on the same axis.
func share(t *testing.T, tr *Tree, id PaneID) float64 {
	t.Helper()
	_, path, ok := tr.find(id)
	if !ok {
		t.Fatalf("pane %d not found", id)
	}
	if len(path) == 0 {
		return 1
	}
	axis := path[len(path)-1].split.Axis
	s := 1.0
	for i := len(path) - 1; i >= 0 && path[i].split.Axis == axis; i-- {
		s *= path[i].split.Ratios[path[i].index]
	}
	return s
}
