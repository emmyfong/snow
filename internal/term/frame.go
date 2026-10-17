package term

// Frame is what a client needs to draw a pane: its rows, styled, its
// cursor, and whether its program asked for the mouse.
type Frame struct {
	Lines   []string
	X, Y    int
	Visible bool // the cursor is shown
	Mouse   bool
}

// Frame returns the pane's frame, read under one lock.
func (p *Pane) Frame() Frame {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frame()
}

// TryFrame returns the pane's frame if no one holds the pane, and false at
// once if output is being written to it. A caller that must never wait,
// such as the server's frame loop, draws a busy pane on a later frame.
func (p *Pane) TryFrame() (Frame, bool) {
	if !p.mu.TryLock() {
		return Frame{}, false
	}
	defer p.mu.Unlock()
	return p.frame(), true
}

// frame reads the frame; the caller holds mu.
func (p *Pane) frame() Frame {
	pos := p.emu.CursorPosition()
	return Frame{Lines: p.screenLines(), X: pos.X, Y: pos.Y, Visible: !p.hidden, Mouse: len(p.mouseModes) > 0}
}
