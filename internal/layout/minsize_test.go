package layout

import (
	"errors"
	"reflect"
	"testing"
)

func TestTooSmallRefused(t *testing.T) {
	// Sizes derive from MinCols and MinRows, so the test survives a change
	// to the minimum. Two panes and a border need 2*min+1 cells.
	wide, tall := 2*MinCols+1, 2*MinRows+1

	tests := []struct {
		name       string
		start      *Tree
		cols, rows int
		target     PaneID
		dir        Direction
		want       *Tree
		wantErr    error
	}{
		{
			name:  "split right fits",
			start: New(1), cols: wide, rows: MinRows,
			target: 1, dir: Right,
			want: &Tree{Root: h(half, leaf(1), leaf(2))},
		},
		{
			name:  "split right one column short",
			start: New(1), cols: wide - 1, rows: MinRows,
			target: 1, dir: Right,
			want: New(1), wantErr: ErrTooSmall,
		},
		{
			name:  "split down fits",
			start: New(1), cols: MinCols, rows: tall,
			target: 1, dir: Down,
			want: &Tree{Root: v(half, leaf(1), leaf(2))},
		},
		{
			name:  "split down one row short",
			start: New(1), cols: MinCols, rows: tall - 1,
			target: 1, dir: Down,
			want: New(1), wantErr: ErrTooSmall,
		},
		{
			// With 16*(wide-1) columns, pane 3's 1/16 share is wide-1 columns.
			name:  "a small pane in a large window",
			start: &Tree{Root: h([]float64{1 - MinRatio, MinRatio}, leaf(1), leaf(3))}, cols: 16 * (wide - 1), rows: 40,
			target: 3, dir: Right,
			want:    &Tree{Root: h([]float64{1 - MinRatio, MinRatio}, leaf(1), leaf(3))},
			wantErr: ErrTooSmall,
		},
		{
			name:  "zoom is ignored for the size check",
			start: &Tree{Root: h(half, leaf(1), leaf(3)), Zoomed: 3}, cols: 2*wide + 1, rows: 40,
			target: 3, dir: Right,
			want: &Tree{Root: h(half, leaf(1), h(half, leaf(3), leaf(2)))},
		},
		{
			name:  "other errors pass through",
			start: New(1), cols: 80, rows: 24,
			target: 9, dir: Right,
			want: New(1), wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.start.SplitWithin(tt.target, tt.dir, 2, tt.cols, tt.rows)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.start, tt.want) {
				t.Fatalf("tree = %s, want %s", dump(t, tt.start), dump(t, tt.want))
			}
		})
	}
}
