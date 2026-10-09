package term

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// ErrReplyPipe means the emulator's reply pipe could not be closed, so a
// goroutine may outlive the pane. It signals an incompatible vt version.
var ErrReplyPipe = errors.New("term: emulator reply pipe is not closable")

// Pane is one program on a PTY plus the screen its output draws. All methods
// are safe for concurrent use.
//
// Two goroutines move bytes: one feeds program output into the emulator, the
// other sends the emulator's encoded keys and replies back to the program.
type Pane struct {
	pty  PTY
	emu  *vt.SafeEmulator
	stop context.CancelFunc
	wg   sync.WaitGroup

	done    chan struct{}
	exitErr error

	closeOnce sync.Once
	closeErr  error
}

// Start runs spec on a new PTY of cols by rows cells.
func Start(spec Spec, cols, rows int, opts Options) (*Pane, error) {
	pty, err := startPTY(spec, cols, rows)
	if err != nil {
		return nil, err
	}
	return newPane(pty, cols, rows, opts), nil
}

func newPane(pty PTY, cols, rows int, opts Options) *Pane {
	emu := vt.NewSafeEmulator(cols, rows)
	emu.SetScrollbackSize(opts.scrollback())
	ctx, stop := context.WithCancel(context.Background())
	p := &Pane{pty: pty, emu: emu, stop: stop, done: make(chan struct{})}

	// The copies end with an error when Close shuts their source; that end is
	// expected, so the errors are dropped.
	p.wg.Add(3)
	go func() { defer p.wg.Done(); _, _ = io.Copy(emu, pty) }()
	// io.Pipe is synchronous: this reader must always drain, or emu.Write
	// blocks while it answers the program's queries.
	go func() { defer p.wg.Done(); _, _ = io.Copy(pty, emu) }()
	go func() {
		defer p.wg.Done()
		p.exitErr = pty.Wait(ctx)
		close(p.done)
	}()
	return p
}

// SendKey encodes k for the program's current terminal mode and sends it.
func (p *Pane) SendKey(k Key) { p.emu.SendKey(k.event()) }

// Paste sends text as a paste, bracketed if the program asked for that.
func (p *Pane) Paste(text string) { p.emu.Paste(text) }

// Resize changes the screen and the PTY to cols by rows cells.
func (p *Pane) Resize(cols, rows int) error {
	p.emu.Resize(cols, rows)
	if err := p.pty.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize pane: %w", err)
	}
	return nil
}

// Size returns the screen size in cells.
func (p *Pane) Size() (cols, rows int) { return p.emu.Width(), p.emu.Height() }

// Render returns the screen with styles as ANSI escape sequences.
func (p *Pane) Render() string { return p.emu.Render() }

// Text returns the screen as plain text.
func (p *Pane) Text() string { return ansi.Strip(p.emu.Render()) }

// Cursor returns the cursor position in cells, from the top left.
func (p *Pane) Cursor() (x, y int) {
	pos := p.emu.CursorPosition()
	return pos.X, pos.Y
}

// Done is closed when the program exits.
func (p *Pane) Done() <-chan struct{} { return p.done }

// Err returns why the program exited. It is valid after Done is closed.
func (p *Pane) Err() error {
	<-p.done
	return p.exitErr
}

// Close kills the program and stops the pane's goroutines. It is safe to call
// more than once.
//
// The order matters. vt's Close is not locked and races with a goroutine
// blocked in Read, so Close wakes that reader by closing the reply pipe
// (which is safe concurrently), waits for every goroutine, and only then
// closes the emulator.
func (p *Pane) Close() error {
	p.closeOnce.Do(func() {
		p.stop()
		ptyErr := p.pty.Close()
		pipeErr := p.closeReplyPipe()
		p.wg.Wait()
		emuErr := p.emu.Close()
		p.closeErr = errors.Join(ptyErr, pipeErr, emuErr)
	})
	return p.closeErr
}

func (p *Pane) closeReplyPipe() error {
	w, ok := p.emu.InputPipe().(*io.PipeWriter)
	if !ok {
		return ErrReplyPipe
	}
	return w.CloseWithError(io.EOF)
}
