// Package layout holds a window's pane layout: a tree whose leaves are panes
// and whose inner nodes split their area between children. It is pure: no
// I/O, no other Snow package, and no goroutines. Callers serialize access.
package layout

// PaneID identifies a pane. IDs start at 1; 0 means none.
type PaneID int

// Node is one node of a layout tree. A leaf has a Pane and nothing else. A
// split has an Axis, two or more Children, and one ratio per child: the
// child's share of the split's area. Ratios are positive and sum to 1.
type Node struct {
	Pane     PaneID    `json:"pane,omitempty"`
	Axis     Axis      `json:"axis,omitempty"`
	Ratios   []float64 `json:"ratios,omitempty"`
	Children []*Node   `json:"children,omitempty"`
}

// Tree is the layout of one window. Zoomed is the pane that fills the window
// while zoom is on, or 0. Change a Tree only through its methods, which keep
// it valid; JSON decoding rejects an invalid tree.
type Tree struct {
	Root   *Node  `json:"root"`
	Zoomed PaneID `json:"zoomed,omitempty"`
}

// New returns a tree with a single pane.
func New(id PaneID) *Tree {
	return &Tree{Root: &Node{Pane: id}}
}

func (n *Node) leaf() bool { return len(n.Children) == 0 }

// Panes returns the tree's pane IDs in order: depth first, left to right and
// top to bottom.
func (t *Tree) Panes() []PaneID {
	var ids []PaneID
	var walk func(*Node)
	walk = func(n *Node) {
		if n.leaf() {
			ids = append(ids, n.Pane)
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	if t.Root != nil {
		walk(t.Root)
	}
	return ids
}

// step is one split on the way from the root to a pane: the split and the
// index of the child that leads on.
type step struct {
	split *Node
	index int
}

// find returns the leaf for id and the splits above it, root first.
func (t *Tree) find(id PaneID) (*Node, []step, bool) {
	var path []step
	var walk func(*Node) *Node
	walk = func(n *Node) *Node {
		if n.leaf() {
			if n.Pane == id {
				return n
			}
			return nil
		}
		for i, c := range n.Children {
			path = append(path, step{n, i})
			if found := walk(c); found != nil {
				return found
			}
			path = path[:len(path)-1]
		}
		return nil
	}
	if t.Root == nil || id <= 0 {
		return nil, nil, false
	}
	n := walk(t.Root)
	return n, path, n != nil
}

// clone returns a deep copy of t, so a caller can try an operation without
// touching the original.
func (t *Tree) clone() *Tree {
	var cp func(*Node) *Node
	cp = func(n *Node) *Node {
		out := &Node{Pane: n.Pane, Axis: n.Axis}
		if n.Ratios != nil {
			out.Ratios = append([]float64(nil), n.Ratios...)
		}
		for _, c := range n.Children {
			out.Children = append(out.Children, cp(c))
		}
		return out
	}
	out := &Tree{Zoomed: t.Zoomed}
	if t.Root != nil {
		out.Root = cp(t.Root)
	}
	return out
}
