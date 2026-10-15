package mouse

import (
	"strconv"

	"github.com/emmyfong/snow/pkg/api"
)

// Drag follows a border that the pointer holds. It turns each pointer move
// into the resize command that moves the border to the pointer.
//
// A resize command grows the target pane toward the side it names, so
// dragging a border toward the pane After it grows the pane Before it, and
// the other way round. The cell count is always positive.
type Drag struct {
	axis          Axis
	before, after int
	pos           int // where the border is believed to be: a column or a row
}

// StartDrag begins a drag at cell x, y when h is a border that runs
// straight between two panes. It returns nil for anything else.
func StartDrag(h Hit, x, y int) *Drag {
	if h.Kind != Border || h.Axis == NoAxis || h.Before == 0 || h.After == 0 {
		return nil
	}
	d := &Drag{axis: h.Axis, before: h.Before, after: h.After}
	d.pos = d.coord(x, y)
	return d
}

func (d *Drag) coord(x, y int) int {
	if d.axis == Vertical {
		return x
	}
	return y
}

// Move returns the command that brings the border to the pointer at x, y,
// or nil when the pointer is on the border's line already. It counts the
// move as done: the next Move measures from here, so a burst of motion
// events does not repeat a resize while the new layout is on its way.
func (d *Drag) Move(x, y int) *api.Command {
	delta := d.coord(x, y) - d.pos
	if delta == 0 {
		return nil
	}
	d.pos += delta
	target, dir := d.before, api.DirRight
	if d.axis == Horizontal {
		dir = api.DirDown
	}
	if delta < 0 {
		delta = -delta
		target, dir = d.after, api.DirLeft
		if d.axis == Horizontal {
			dir = api.DirUp
		}
	}
	return &api.Command{Op: api.OpResize, Target: target,
		Args: map[string]string{api.ArgDir: dir, api.ArgCells: strconv.Itoa(delta)}}
}

// Sync puts the drag back in step with a new layout: the border sits where
// the layout says, whatever was asked for. A resize the server refused, or
// clamped at a minimum size, then costs nothing. It reports false when the
// pane before the border is gone, which ends the drag.
func (d *Drag) Sync(panes []api.PaneInfo) bool {
	for _, p := range panes {
		if p.ID != d.before {
			continue
		}
		if d.axis == Vertical {
			d.pos = p.Rect.X + p.Rect.W
		} else {
			d.pos = p.Rect.Y + p.Rect.H
		}
		return true
	}
	return false
}
