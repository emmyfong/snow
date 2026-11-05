package layout

import (
	"fmt"
	"math"
)

// Template is a window layout by pane kind: a tree shaped like Node whose
// leaves name a kind instead of a pane. A preset or a saved project layout
// is a Template; Build turns it into a Tree for the panes that fill it.
type Template struct {
	Kind     string     `json:"kind,omitempty"`
	Axis     Axis       `json:"axis,omitempty"`
	Ratios   []float64  `json:"ratios,omitempty"`
	Children []Template `json:"children,omitempty"`
}

// Kinds returns the leaves' kinds in the order Tree.Panes lists panes:
// depth first, left to right and top to bottom.
func (t Template) Kinds() []string {
	if len(t.Children) == 0 {
		return []string{t.Kind}
	}
	var out []string
	for _, c := range t.Children {
		out = append(out, c.Kinds()...)
	}
	return out
}

// Validate reports the first broken rule of t, wrapped in ErrInvalidTree:
// a leaf has a kind and nothing else; a split has an axis, two or more
// children, and one positive ratio per child, summing to 1.
func (t Template) Validate() error {
	if len(t.Children) == 0 {
		if t.Kind == "" || t.Axis != 0 || t.Ratios != nil {
			return fmt.Errorf("%w: template leaf %q needs a kind and nothing else", ErrInvalidTree, t.Kind)
		}
		return nil
	}
	switch {
	case t.Kind != "":
		return fmt.Errorf("%w: template split has kind %q", ErrInvalidTree, t.Kind)
	case t.Axis != Horizontal && t.Axis != Vertical:
		return fmt.Errorf("%w: template split axis %d", ErrInvalidTree, int(t.Axis))
	case len(t.Children) < 2:
		return fmt.Errorf("%w: template split has %d child", ErrInvalidTree, len(t.Children))
	case len(t.Ratios) != len(t.Children):
		return fmt.Errorf("%w: %d ratios for %d children", ErrInvalidTree, len(t.Ratios), len(t.Children))
	}
	var sum float64
	for _, r := range t.Ratios {
		if !(r > 0) || math.IsInf(r, 0) {
			return fmt.Errorf("%w: ratio %v", ErrInvalidTree, r)
		}
		sum += r
	}
	if math.Abs(sum-1) > ratioTolerance {
		return fmt.Errorf("%w: ratios sum to %v", ErrInvalidTree, sum)
	}
	for _, c := range t.Children {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Prune returns t without the leaves whose kind keep refuses. A split's
// remaining children share its area in their old proportions, and a split
// left with one child becomes that child. ok is false when no leaf is kept.
func (t Template) Prune(keep func(kind string) bool) (pruned Template, ok bool) {
	if len(t.Children) == 0 {
		return t, keep(t.Kind)
	}
	var children []Template
	var ratios []float64
	var sum float64
	for i, c := range t.Children {
		if pc, ok := c.Prune(keep); ok {
			children = append(children, pc)
			ratios = append(ratios, t.Ratios[i])
			sum += t.Ratios[i]
		}
	}
	switch len(children) {
	case 0:
		return Template{}, false
	case 1:
		return children[0], true
	}
	for i := range ratios {
		ratios[i] /= sum
	}
	return Template{Axis: t.Axis, Ratios: ratios, Children: children}, true
}

// Build returns the Tree of t with ids as its panes, one per leaf in Kinds
// order. It refuses with ErrTooSmall when a pane would be smaller than
// MinCols by MinRows in a window of cols by rows cells.
func (t Template) Build(ids []PaneID, cols, rows int) (*Tree, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if n := len(t.Kinds()); len(ids) != n {
		return nil, fmt.Errorf("%w: %d panes for a template of %d", ErrInvalidTree, len(ids), n)
	}
	next := 0
	var build func(t Template) *Node
	build = func(t Template) *Node {
		if len(t.Children) == 0 {
			next++
			return &Node{Pane: ids[next-1]}
		}
		n := &Node{Axis: t.Axis, Ratios: append([]float64(nil), t.Ratios...)}
		for _, c := range t.Children {
			n.Children = append(n.Children, build(c))
		}
		return n
	}
	tree := &Tree{Root: build(t)}
	if err := tree.validate(); err != nil {
		return nil, err
	}
	for id, r := range place(tree.Root, Rect{W: max(cols, 0), H: max(rows, 0)}).Panes {
		if r.W < MinCols || r.H < MinRows {
			return nil, fmt.Errorf("%w: pane %d would be %dx%d in a %dx%d window, minimum %dx%d",
				ErrTooSmall, id, r.W, r.H, cols, rows, MinCols, MinRows)
		}
	}
	return tree, nil
}

// Apply replaces t's layout with tmpl built for ids, as Build does. On an
// error t is unchanged.
func (t *Tree) Apply(tmpl Template, ids []PaneID, cols, rows int) error {
	built, err := tmpl.Build(ids, cols, rows)
	if err != nil {
		return err
	}
	*t = *built
	return nil
}
