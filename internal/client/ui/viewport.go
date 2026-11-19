package ui

import tea "charm.land/bubbletea/v2"

// Viewport is the scrolling body shared by a read-only side pane: a header
// row, the body's rows, and a hints row.
type Viewport struct {
	Width  int
	Height int
	Top    int // the first visible body row
}

// SetSize sets the viewport's size in cells.
func (v *Viewport) SetSize(width, height int) {
	v.Width, v.Height = width, height
}

// BodyHeight is the rows the body shows, below the header and above the
// hints.
func (v *Viewport) BodyHeight() int { return max(v.Height-2, 0) }

// Clamp keeps the first visible row inside a body of lines rows.
func (v *Viewport) Clamp(lines int) {
	v.Top = Scroll(v.Top, 0, lines, v.BodyHeight())
}

// ScrollBy moves the first visible row by rows in a body of lines rows.
func (v *Viewport) ScrollBy(rows, lines int) {
	v.Top = Scroll(v.Top, rows, lines, v.BodyHeight())
}

// Key handles a key that moves a body of lines rows. It reports whether the
// key asked for a refresh and whether it handled the key at all.
func (v *Viewport) Key(k tea.Key, lines int) (refresh, handled bool) {
	page := max(v.BodyHeight()-1, 1)
	switch {
	case k.Code == tea.KeyUp || k.Text == "k":
		v.ScrollBy(-1, lines)
	case k.Code == tea.KeyDown || k.Text == "j":
		v.ScrollBy(1, lines)
	case k.Code == tea.KeyPgUp:
		v.ScrollBy(-page, lines)
	case k.Code == tea.KeyPgDown:
		v.ScrollBy(page, lines)
	case k.Code == tea.KeyHome || k.Text == "g":
		v.Top = 0
	case k.Code == tea.KeyEnd || k.Text == "G":
		v.Top = lines
		v.Clamp(lines)
	case k.Text == "r":
		return true, true
	default:
		return false, false
	}
	return false, true
}
