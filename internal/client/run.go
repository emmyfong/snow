package client

import (
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/pkg/api"
)

// Options configures a client run.
type Options struct {
	Version string
	Socket  string
	Session string       // the session to attach to or create; "" creates a numbered one
	Dir     string       // the folder a new session starts in; "" means the current one
	Start   func() error // starts a server when none answers
	Input   io.Reader    // nil means standard input
	Output  io.Writer    // nil means standard output
}

// outboxSize bounds queued messages to the server. Keys from fast typing fit
// many times over.
const outboxSize = 1024

// Run connects to the server, attaches, and shows the session until the user
// detaches or the connection closes.
func Run(opts Options) (Result, error) {
	raw, err := Connect(opts.Socket, opts.Start)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = raw.Close() }()
	conn := api.NewConn(raw)

	// terminalSize falls back to 80x24, so the size is always known here.
	cols, rows, _ := acceptedSize(terminalSize(opts.Output))
	hello := api.Hello{ClientVersion: opts.Version, Protocol: api.ProtocolVersion, Cols: cols, Rows: rows}
	if _, err := api.ClientHandshake(conn, hello); err != nil {
		return Result{}, fmt.Errorf("connect to server: %w", err)
	}

	// One goroutine writes every message in order. Sending from tea.Cmds
	// would run them concurrently and could reorder fast typing.
	out := make(chan api.Message, outboxSize)
	go func() {
		for m := range out {
			if conn.Send(m) != nil {
				return
			}
		}
	}()
	send := func(m api.Message) { out <- m }
	defer close(out)

	dir := opts.Dir
	if dir == "" {
		// Without a folder the session starts in the home folder; an error
		// here only loses that convenience.
		dir, _ = os.Getwd()
	}
	send(&api.Attach{Session: opts.Session, Create: true, Dir: dir})
	a := &app{screen: session.New(send), send: send}
	a.screen.SetSize(cols, rows)

	var popts []tea.ProgramOption
	if opts.Input != nil {
		popts = append(popts, tea.WithInput(opts.Input))
	}
	if opts.Output != nil {
		popts = append(popts, tea.WithOutput(opts.Output))
	}
	p := tea.NewProgram(a, popts...)
	go receive(conn, p)
	if _, err := p.Run(); err != nil {
		return a.result, fmt.Errorf("run screen: %w", err)
	}
	return a.result, nil
}

// receive delivers server messages to the program as Bubble Tea messages, so
// no goroutine but the program's touches the model.
func receive(conn *api.Conn, p *tea.Program) {
	for {
		m, err := conn.Receive()
		if errors.Is(err, api.ErrUnknownType) || errors.Is(err, api.ErrBadPayload) {
			continue
		}
		if err != nil {
			p.Send(connClosedMsg{})
			return
		}
		p.Send(m)
	}
}

// terminalSize reads the size of the terminal behind out, or 80x24.
func terminalSize(out io.Writer) (cols, rows int) {
	f, ok := out.(*os.File)
	if out == nil {
		f, ok = os.Stdout, true
	}
	if ok {
		if w, h, err := term.GetSize(f.Fd()); err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	return 80, 24
}
