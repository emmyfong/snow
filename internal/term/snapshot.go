package term

import "github.com/charmbracelet/x/ansi"

// Snapshot is a plain-text copy of a pane's screen and scrollback, with its
// cursor and size, taken at one moment. Lines carry no styles.
type Snapshot struct {
	// Screen holds the rows now on the screen, top row first.
	Screen []string
	// Scrollback holds up to the requested number of newest lines above
	// the screen, oldest first.
	Scrollback []string
	// Omitted counts the scrollback lines older than the ones returned.
	Omitted int
	// CursorX and CursorY are the cursor's cell, from the top left.
	CursorX, CursorY int
	// CursorVisible is set while the program shows the cursor.
	CursorVisible bool
	Cols, Rows    int
}

// Snapshot copies the pane's screen and the newest scrollback lines as
// plain text, with its cursor and size. It copies under the pane's lock and
// strips styles after it releases the lock, so a large read never holds up
// the program's output for long.
func (p *Pane) Snapshot(scrollback int) Snapshot {
	p.mu.Lock()
	sb := p.emu.Scrollback()
	kept := sb.Len()
	n := min(max(scrollback, 0), kept)
	back := make([]string, 0, n)
	for i := kept - n; i < kept; i++ {
		back = append(back, sb.Line(i).Render())
	}
	pos := p.emu.CursorPosition()
	snap := Snapshot{
		Omitted:       kept - n,
		CursorX:       pos.X,
		CursorY:       pos.Y,
		CursorVisible: !p.hidden,
		Cols:          p.emu.Width(),
		Rows:          p.emu.Height(),
	}
	screen := p.screenLines()
	p.mu.Unlock()

	for i, line := range back {
		back[i] = ansi.Strip(line)
	}
	for i, line := range screen {
		screen[i] = ansi.Strip(line)
	}
	snap.Screen, snap.Scrollback = screen, back
	return snap
}
