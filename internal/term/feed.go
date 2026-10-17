package term

import "runtime"

// feed parses program output into the emulator until the PTY closes.
func (p *Pane) feed() {
	// feed writes the preamble, not newPane: a write can make the emulator
	// reply, and replies wait for drain, which runs by now.
	if p.preamble != "" {
		p.mu.Lock()
		_, _ = p.emu.Write([]byte(p.preamble)) // the emulator's writer never fails
		p.mu.Unlock()
		p.version.Add(1)
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := p.pty.Read(buf)
		// Feed the emulator a piece at a time, releasing mu between pieces:
		// scrolling a full scrollback costs the emulator time for every
		// line, and a reader of the screen must never wait for a whole read.
		for chunk := buf[:n]; len(chunk) > 0; {
			piece := chunk[:pieceEnd(chunk)]
			chunk = chunk[len(piece):]
			p.mu.Lock()
			p.shift.before(p.emu.Scrollback())
			_, _ = p.emu.Write(piece) // the emulator's writer never fails
			p.shift.after(p.emu.Scrollback())
			p.mu.Unlock()
			p.version.Add(1)
			// A pane printing without pause would keep its goroutine busy
			// for a whole scheduler slice; yielding lets a pane that only
			// echoed a key, and the server's frame loop, run at once.
			runtime.Gosched()
		}
		if err != nil {
			return
		}
	}
}

// feedPiece is about how much output feed writes to the emulator under one
// hold of mu.
const feedPiece = 1024

// pieceEnd returns where the first piece of chunk ends: just before the
// first ASCII byte at or after feedPiece, or the whole chunk. The emulator
// ends the character it is building at the end of each write, so a piece
// must not end inside one: "e" then a combining accent in the next write
// would lose the accent. An ASCII byte is never part of a multi-byte rune
// and always starts a new character. Escape sequences may still be split;
// the emulator keeps its parse state for those, as PTY reads split them too.
func pieceEnd(chunk []byte) int {
	for i := min(feedPiece, len(chunk)); i < len(chunk); i++ {
		if chunk[i] < 0x80 {
			return i
		}
	}
	return len(chunk)
}
