package mouse

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func resizeCmd(target int, dir string, cells string) api.Command {
	return api.Command{Op: api.OpResize, Target: target, Args: map[string]string{api.ArgDir: dir, api.ArgCells: cells}}
}

func same(a, b *api.Command) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Op != b.Op || a.Target != b.Target || len(a.Args) != len(b.Args) {
		return false
	}
	for k, v := range a.Args {
		if b.Args[k] != v {
			return false
		}
	}
	return true
}

// TestDragResizes: a border drag becomes a resize command that moves the
// border as far as the pointer moved. A positive resize grows the pane on
// one side, so a drag toward a pane grows the pane behind it.
func TestDragResizes(t *testing.T) {
	const cols, rows = 80, 24
	tests := []struct {
		name       string
		startX, Y  int
		toX, toY   int
		want       api.Command
		wantNoMove bool
	}{
		{"right", 40, 5, 45, 5, resizeCmd(1, api.DirRight, "5"), false},
		{"left", 40, 5, 37, 5, resizeCmd(2, api.DirLeft, "3"), false},
		{"down", 60, 12, 60, 15, resizeCmd(2, api.DirDown, "3"), false},
		{"up", 60, 12, 60, 8, resizeCmd(3, api.DirUp, "4"), false},
		{"the other axis is ignored", 40, 5, 40, 9, api.Command{}, true},
		{"no move", 40, 5, 40, 5, api.Command{}, true},
	}
	for _, tt := range tests {
		h := HitTest(threePanes(), tabs, tt.startX, tt.Y, cols, rows)
		d := StartDrag(h, tt.startX, tt.Y)
		if d == nil {
			t.Fatalf("%s: no drag started on %+v", tt.name, h)
		}
		got := d.Move(tt.toX, tt.toY)
		if tt.wantNoMove {
			if got != nil {
				t.Errorf("%s: Move = %+v, want nil", tt.name, got)
			}
			continue
		}
		if !same(got, &tt.want) {
			t.Errorf("%s: Move = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestDragFollowsThePointerStepByStep(t *testing.T) {
	d := StartDrag(HitTest(threePanes(), nil, 40, 5, 80, 24), 40, 5)
	steps := []struct {
		x    int
		want *api.Command
	}{
		{42, cmdPtr(resizeCmd(1, api.DirRight, "2"))},
		{42, nil},
		{41, cmdPtr(resizeCmd(2, api.DirLeft, "1"))},
		{45, cmdPtr(resizeCmd(1, api.DirRight, "4"))},
	}
	for i, s := range steps {
		if got := d.Move(s.x, 5); !same(got, s.want) {
			t.Errorf("step %d to x=%d: %+v, want %+v", i, s.x, got, s.want)
		}
	}
}

func cmdPtr(c api.Command) *api.Command { return &c }

// A refused resize leaves the border where it was. Sync puts the drag back
// in step with the layout, so the next move asks for the whole distance.
func TestDragSyncsWithTheLayout(t *testing.T) {
	d := StartDrag(HitTest(threePanes(), nil, 40, 5, 80, 24), 40, 5)
	d.Move(50, 5) // the server refuses: the layout stays as it was
	if !d.Sync(threePanes()) {
		t.Fatal("Sync reported the drag over")
	}
	got := d.Move(50, 5)
	if !same(got, cmdPtr(resizeCmd(1, api.DirRight, "10"))) {
		t.Errorf("after Sync, Move = %+v, want the full 10 cells", got)
	}

	// The border moved to column 48.
	moved := threePanes()
	moved[0].Rect.W = 48
	moved[1].Rect.X, moved[1].Rect.W = 49, 31
	moved[2].Rect.X, moved[2].Rect.W = 49, 31
	d.Sync(moved)
	if got := d.Move(50, 5); !same(got, cmdPtr(resizeCmd(1, api.DirRight, "2"))) {
		t.Errorf("after the border moved, Move = %+v, want 2 cells", got)
	}
}

func TestDragEndsWhenItsPaneIsGone(t *testing.T) {
	d := StartDrag(HitTest(threePanes(), nil, 40, 5, 80, 24), 40, 5)
	if d.Sync(threePanes()[1:]) {
		t.Error("Sync kept a drag whose pane is gone")
	}
}

func TestStartDragOnlyOnBorders(t *testing.T) {
	for _, h := range []Hit{{Kind: Pane, Pane: 1}, {Kind: Tab}, {Kind: Bar}, {Kind: None}, {Kind: Border}} {
		if d := StartDrag(h, 1, 1); d != nil {
			t.Errorf("StartDrag(%+v) started a drag", h)
		}
	}
}
