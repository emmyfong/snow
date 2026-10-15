package layout

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

// leaf, h, and v build trees in the notation of the issues: h(1,v(2,3)).
// h and v copy ratios, because operations change ratios in place and test
// cases share slices such as half.
func leaf(id PaneID) *Node { return &Node{Pane: id} }

func h(ratios []float64, children ...*Node) *Node {
	return &Node{Axis: Horizontal, Ratios: slices.Clone(ratios), Children: children}
}

func v(ratios []float64, children ...*Node) *Node {
	return &Node{Axis: Vertical, Ratios: slices.Clone(ratios), Children: children}
}

var half = []float64{0.5, 0.5}

func TestTreeOps(t *testing.T) {
	tests := []struct {
		name    string
		start   *Tree
		op      func(*Tree) error
		want    *Tree
		wantErr error
	}{
		{
			name:  "split right",
			start: New(1),
			op:    func(t *Tree) error { return t.Split(1, Right, 2) },
			want:  &Tree{Root: h(half, leaf(1), leaf(2))},
		},
		{
			name:  "split down",
			start: New(1),
			op:    func(t *Tree) error { return t.Split(1, Down, 2) },
			want:  &Tree{Root: v(half, leaf(1), leaf(2))},
		},
		{
			name:  "split left puts the new pane first",
			start: New(1),
			op:    func(t *Tree) error { return t.Split(1, Left, 2) },
			want:  &Tree{Root: h(half, leaf(2), leaf(1))},
		},
		{
			name:  "split up puts the new pane first",
			start: New(1),
			op:    func(t *Tree) error { return t.Split(1, Up, 2) },
			want:  &Tree{Root: v(half, leaf(2), leaf(1))},
		},
		{
			name:  "split nests inside a split on the same axis",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Split(2, Right, 3) },
			want:  &Tree{Root: h(half, leaf(1), h(half, leaf(2), leaf(3)))},
		},
		{
			name:    "split an unknown pane",
			start:   New(1),
			op:      func(t *Tree) error { return t.Split(9, Right, 2) },
			want:    New(1),
			wantErr: ErrNotFound,
		},
		{
			name:    "split with a pane ID already in the tree",
			start:   &Tree{Root: h(half, leaf(1), leaf(2))},
			op:      func(t *Tree) error { return t.Split(1, Down, 2) },
			want:    &Tree{Root: h(half, leaf(1), leaf(2))},
			wantErr: ErrDuplicatePane,
		},
		{
			name:    "split with pane ID zero",
			start:   New(1),
			op:      func(t *Tree) error { return t.Split(1, Right, 0) },
			want:    New(1),
			wantErr: ErrInvalidPane,
		},
		{
			name:    "split with an invalid direction",
			start:   New(1),
			op:      func(t *Tree) error { return t.Split(1, Direction(0), 2) },
			want:    New(1),
			wantErr: ErrInvalidDirection,
		},
		{
			name:  "split unzooms",
			start: &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 1},
			op:    func(t *Tree) error { return t.Split(2, Down, 3) },
			want:  &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))},
		},
		{
			name:  "close collapses a one-child split",
			start: &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))},
			op:    func(t *Tree) error { return t.Close(3) },
			want:  &Tree{Root: h(half, leaf(1), leaf(2))},
		},
		{
			name:  "close collapses the root split",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Close(1) },
			want:  New(2),
		},
		{
			name:  "close gives the share to the previous sibling",
			start: &Tree{Root: h([]float64{0.25, 0.25, 0.5}, leaf(1), leaf(2), leaf(3))},
			op:    func(t *Tree) error { return t.Close(3) },
			want:  &Tree{Root: h([]float64{0.25, 0.75}, leaf(1), leaf(2))},
		},
		{
			name:  "close of the first child gives the share to the next sibling",
			start: &Tree{Root: h([]float64{0.25, 0.25, 0.5}, leaf(1), leaf(2), leaf(3))},
			op:    func(t *Tree) error { return t.Close(1) },
			want:  &Tree{Root: h([]float64{0.5, 0.5}, leaf(2), leaf(3))},
		},
		{
			name:    "close the last pane",
			start:   New(1),
			op:      func(t *Tree) error { return t.Close(1) },
			want:    New(1),
			wantErr: ErrLastPane,
		},
		{
			name:    "close an unknown pane",
			start:   &Tree{Root: h(half, leaf(1), leaf(2))},
			op:      func(t *Tree) error { return t.Close(9) },
			want:    &Tree{Root: h(half, leaf(1), leaf(2))},
			wantErr: ErrNotFound,
		},
		{
			name:  "close unzooms",
			start: &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 1},
			op:    func(t *Tree) error { return t.Close(2) },
			want:  New(1),
		},
		{
			name:  "resize grows toward the side",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Resize(1, Right, 0.25) },
			want:  &Tree{Root: h([]float64{0.75, 0.25}, leaf(1), leaf(2))},
		},
		{
			name:  "resize with a negative delta shrinks",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Resize(2, Left, -0.25) },
			want:  &Tree{Root: h([]float64{0.75, 0.25}, leaf(1), leaf(2))},
		},
		{
			name:  "resize clamps the grown pane",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Resize(1, Right, 2) },
			want:  &Tree{Root: h([]float64{1 - MinRatio, MinRatio}, leaf(1), leaf(2))},
		},
		{
			name:  "resize clamps the shrunk pane",
			start: &Tree{Root: v(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Resize(1, Down, -2) },
			want:  &Tree{Root: v([]float64{MinRatio, 1 - MinRatio}, leaf(1), leaf(2))},
		},
		{
			name:  "resize uses the nearest split on the axis",
			start: &Tree{Root: h(half, leaf(1), v(half, leaf(2), h(half, leaf(3), leaf(4))))},
			op:    func(t *Tree) error { return t.Resize(3, Left, 0.25) },
			want:  &Tree{Root: h([]float64{0.25, 0.75}, leaf(1), v(half, leaf(2), h(half, leaf(3), leaf(4))))},
		},
		{
			name:    "resize at the window edge",
			start:   &Tree{Root: h(half, leaf(1), leaf(2))},
			op:      func(t *Tree) error { return t.Resize(2, Right, 0.25) },
			want:    &Tree{Root: h(half, leaf(1), leaf(2))},
			wantErr: ErrNoEdge,
		},
		{
			name:    "resize across the wrong axis",
			start:   &Tree{Root: h(half, leaf(1), leaf(2))},
			op:      func(t *Tree) error { return t.Resize(1, Down, 0.25) },
			want:    &Tree{Root: h(half, leaf(1), leaf(2))},
			wantErr: ErrNoEdge,
		},
		{
			name:    "resize an unknown pane",
			start:   New(1),
			op:      func(t *Tree) error { return t.Resize(9, Right, 0.25) },
			want:    New(1),
			wantErr: ErrNotFound,
		},
		{
			name:  "resize unzooms",
			start: &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2},
			op:    func(t *Tree) error { return t.Resize(1, Right, 0.25) },
			want:  &Tree{Root: h([]float64{0.75, 0.25}, leaf(1), leaf(2))},
		},
		{
			name:  "zoom",
			start: &Tree{Root: h(half, leaf(1), leaf(2))},
			op:    func(t *Tree) error { return t.Zoom(2) },
			want:  &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2},
		},
		{
			name:  "zoom moves to another pane",
			start: &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2},
			op:    func(t *Tree) error { return t.Zoom(1) },
			want:  &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 1},
		},
		{
			name:    "zoom an unknown pane",
			start:   New(1),
			op:      func(t *Tree) error { return t.Zoom(9) },
			want:    New(1),
			wantErr: ErrNotFound,
		},
		{
			name:  "unzoom",
			start: &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2},
			op:    func(t *Tree) error { t.Unzoom(); return nil },
			want:  &Tree{Root: h(half, leaf(1), leaf(2))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op(tt.start)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.start, tt.want) {
				t.Fatalf("tree = %s, want %s", dump(t, tt.start), dump(t, tt.want))
			}
			if err := tt.start.validate(); err != nil {
				t.Fatalf("tree is invalid after the operation: %v", err)
			}
		})
	}
}

func TestPanes(t *testing.T) {
	tr := &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}
	if got, want := tr.Panes(), []PaneID{1, 2, 3}; !slices.Equal(got, want) {
		t.Fatalf("Panes() = %v, want %v", got, want)
	}
}

// dump renders a tree as JSON for failure messages.
func dump(t *testing.T, tr *Tree) string {
	t.Helper()
	b, err := tr.MarshalJSON()
	if err != nil {
		return err.Error()
	}
	return string(b)
}
