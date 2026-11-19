package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/pkg/api"
)

func TestAreaReservesWindowBar(t *testing.T) {
	for _, tt := range []struct{ rows, want int }{{24, 21}, {40, 37}, {3, 1}, {1, 1}} {
		sess := &session{cols: 80, rows: tt.rows}
		cols, rows := sess.area()
		if cols != 80 || rows != tt.want {
			t.Fatalf("screen 80x%d: content canvas %dx%d,want 80x%d", tt.rows, cols, rows, tt.want)
		}
	}
}

func TestPaneHeadersReuseDividers(t *testing.T) {
	tree := layout.New(1)
	if err := tree.Split(1, layout.Down, 2); err != nil {
		t.Fatal(err)
	}
	if err := tree.Split(1, layout.Right, 3); err != nil {
		t.Fatal(err)
	}
	sess := &session{cols: 80, rows: 24, windows: []*window{{tree: tree, focused: 1, panes: map[int]*pane{1: {id: 1}, 2: {id: 2}, 3: {id: 3}}}}}
	st := &state{}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {13, 9}} {
		sess.cols, sess.rows = size[0], size[1]
		cols, rows := sess.area()
		logical := layout.Rects(tree, cols, rows)
		got := st.layout(sess).Windows[0].Panes
		for _, p := range got {
			raw := logical.Panes[layout.PaneID(p.ID)]
			if p.Rect != (api.Rect{X: raw.X, Y: raw.Y + 1, W: raw.W, H: raw.H}) {
				t.Fatalf("size %v: content rectangle %+v differs from logical %+v", size, p.Rect, raw)
			}
			if p.Rect.Y < 1 || p.Rect.W < 1 || p.Rect.H < 1 || p.Rect.Y+p.Rect.H > size[1]-2 {
				t.Fatalf("size %v: invalid content rectangle %+v", size, p.Rect)
			}
		}
		for _, p := range got {
			if p.ID == 2 && p.Rect.Y <= 1 {
				t.Fatal("lower pane lost its divider header")
			}
			for _, other := range got {
				if p.ID != other.ID && rectanglesOverlap(p.Rect, other.Rect) {
					t.Fatalf("panes overlap: %+v %+v", p.Rect, other.Rect)
				}
			}
		}
	}
	if err := tree.Zoom(2); err != nil {
		t.Fatal(err)
	}
	got := st.layout(sess).Windows[0].Panes
	if len(got) != 1 || got[0].Rect != (api.Rect{Y: 1, W: 13, H: 6}) {
		t.Fatalf("zoomed content %+v", got)
	}
}

func rectanglesOverlap(a, b api.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestHeaderContentSizesFollowResize(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	c.Send(&api.Resize{Cols: 120, Rows: 40})
	l := nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].Rect.W == 120 })
	if got := shownPanes(l)[0].Rect; got != (api.Rect{Y: 1, W: 120, H: 37}) {
		t.Fatalf("resized content %+v", got)
	}
	if cols, rows := s.pane(t, "work").Size(); cols != 120 || rows != 37 {
		t.Fatalf("PTY %dx%d,want 120x37", cols, rows)
	}
	c.Send(split(first, api.DirDown))
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	ps := shownPanes(l)
	if ps[0].Rect.Y != 1 || ps[0].Rect.H != 18 || ps[1].Rect.Y != 20 || ps[1].Rect.H != 18 {
		t.Fatalf("split headers/content %+v", ps)
	}
	c.Send(&api.Command{Op: api.OpResize, Target: first, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgCells: "3"}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].Rect.H == 21 })
	ps = shownPanes(l)
	if ps[1].Rect.Y != 23 || ps[1].Rect.H != 15 {
		t.Fatalf("moved divider/header %+v", ps)
	}
	if cols, rows := s.pane(t, "work").Size(); cols != 120 || rows != 15 {
		t.Fatalf("split PTY %dx%d,want 120x15", cols, rows)
	}
}

func TestHeaderMinimumSplitKeepsContent(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 20, 6)
	c.Send(split(first, api.DirDown))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Rect.H != 1 {
			t.Fatalf("minimum content %+v", p.Rect)
		}
	}
	c.Send(split(first, api.DirDown))
	if e := nextError(t, c); e.Code != api.CodeCommandFailed {
		t.Fatalf("too-small split %+v", e)
	}
	if cols, rows := s.pane(t, "work").Size(); cols != 20 || rows != 1 {
		t.Fatalf("minimum PTY %dx%d,want 20x1", cols, rows)
	}
	// A failed split keeps the existing layout and its usable content rows.
	c.Send(&api.Command{Op: api.OpZoom, Target: first})
	l = nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
	if ps := shownPanes(l); len(ps) != 1 || ps[0].Rect.Y != 1 || ps[0].Rect.H != 3 {
		t.Fatalf("minimum zoom %+v", ps)
	}
}
