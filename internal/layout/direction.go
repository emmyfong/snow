package layout

import "fmt"

// Axis is the direction in which a split lays out its children. The zero
// value means none and marks a leaf.
type Axis int

// Split axes.
const (
	// Horizontal places children side by side, left to right.
	Horizontal Axis = iota + 1
	// Vertical stacks children top to bottom.
	Vertical
)

// MarshalText writes the axis as "horizontal" or "vertical", so snapshots
// stay readable.
func (a Axis) MarshalText() ([]byte, error) {
	switch a {
	case Horizontal:
		return []byte("horizontal"), nil
	case Vertical:
		return []byte("vertical"), nil
	}
	return nil, fmt.Errorf("%w: %d", ErrInvalidAxis, int(a))
}

// UnmarshalText reads an axis written by MarshalText.
func (a *Axis) UnmarshalText(b []byte) error {
	switch string(b) {
	case "horizontal":
		*a = Horizontal
	case "vertical":
		*a = Vertical
	default:
		return fmt.Errorf("%w: %q", ErrInvalidAxis, b)
	}
	return nil
}

// Direction is a side of a pane: where a split puts the new pane, which edge
// a resize moves, or where to look for a neighbor.
type Direction int

// Directions. The zero value is invalid, so a forgotten argument fails.
const (
	Left Direction = iota + 1
	Right
	Up
	Down
)

func (d Direction) valid() bool { return d >= Left && d <= Down }

// axis returns the split axis that a move toward d crosses.
func (d Direction) axis() Axis {
	if d == Left || d == Right {
		return Horizontal
	}
	return Vertical
}

// forward reports whether d points toward later children: right or down.
func (d Direction) forward() bool { return d == Right || d == Down }
