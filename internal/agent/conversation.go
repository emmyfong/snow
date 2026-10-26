// Package agent runs Snow's own agent: a conversation that sends turns to
// a provider, runs the tools the model calls, saves every record to its
// conversation file before anything depends on it, and reports what it
// does as events.
package agent

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
)

// Config is what a Conversation needs from the server.
type Config struct {
	Store     *convstore.Store
	Providers *provider.Registry
	Tools     *tool.Registry
	// Folder is the conversation folder. For a new conversation it is also
	// saved in the meta record.
	Folder tool.Folder
	// Shell is the profile the run tool uses. A new conversation saves its
	// name in the meta record.
	Shell shell.Profile
	// HideEnv names environment variables the run tool's commands do not
	// get: the variables that hold provider keys. The server fills it, as
	// the agent does not read keys.
	HideEnv []string
	System  string // the system prompt; empty means DefaultSystem
	// Events receives every event, in order, from one goroutine of the
	// Conversation's own. A slow receiver delays later events, never the
	// turn. It must not call Close, which waits for that goroutine.
	Events func(Event)
	Now    func() time.Time // the clock; nil means time.Now
}

// Errors a Conversation returns. Match them with errors.Is.
var (
	// ErrClosed is the cause of a call to a closed Conversation.
	ErrClosed = errors.New("conversation is closed")
	// ErrInvalid is the cause of a call with a mode, a decision, or a model
	// name this version does not know.
	ErrInvalid = errors.New("invalid argument")
	// ErrBusy is the cause of a Retry while a turn runs or messages wait.
	ErrBusy = errors.New("a turn is running")
	// ErrFolderMismatch is the cause of a Resume whose folder is not the one
	// saved in the conversation file: its tools would work elsewhere.
	ErrFolderMismatch = errors.New("folder differs from the conversation's folder")
)

// Conversation is one conversation with its main agent. One goroutine owns
// its state; the methods only post commands to it, so they are safe to
// call from any goroutine and never wait for a turn.
type Conversation struct {
	cfg   Config
	id    string
	agent ID
	w     *convstore.Writer
	ev    *emitter
	cmds  chan any
	done  chan struct{}

	// Used by the goroutine that runs the current turn. Turns never
	// overlap, and the loop changes the model only between turns.
	history []provider.Message
	model   string // "provider/model"
	effort  provider.Effort
	grants  map[string]bool // tools the user said yes to for this conversation
	unsaved string          // the user's message whose save failed; a retry sends it
	turns   int             // turns saved before this run; the loop numbers new ones after them
	open    *Question       // the question a resumed file left open; the loop takes it

	mode atomic.Value // Mode; set by the loop, read by the turn
}

// Start creates a new conversation with meta, saves its meta record, and
// starts its goroutine. Meta.Folder is taken from cfg.Folder; an unset
// Meta.Profile is cfg.Shell's name, and an unset Meta.Created is now.
func Start(cfg Config, meta convstore.Meta) (*Conversation, error) {
	meta.Folder = cfg.Folder.Path
	if meta.Profile == "" {
		meta.Profile = cfg.Shell.Name
	}
	if meta.Created.IsZero() {
		meta.Created = now(cfg)
	}
	w, err := cfg.Store.Create(meta)
	if err != nil {
		return nil, fmt.Errorf("start conversation: %w", err)
	}
	c := newConversation(cfg, w)
	c.model, c.effort = meta.Model, provider.Effort(meta.Effort)
	go c.loop()
	return c, nil
}

// Resume loads the conversation id, ends a turn that a crash left open,
// and starts its goroutine. It returns what was loaded, for the caller to
// show. The conversation goes on with the model and the mode saved in its
// file. cfg.Folder must be the folder saved in the file, or Resume returns
// an error wrapping ErrFolderMismatch.
func Resume(cfg Config, id string) (*Conversation, *convstore.Conversation, error) {
	loaded, err := cfg.Store.Load(id)
	if err != nil {
		return nil, nil, fmt.Errorf("resume conversation: %w", err)
	}
	return ResumeLoaded(cfg, loaded)
}

// ResumeLoaded is Resume for a conversation the caller loaded already, for
// a caller that needs the file's folder before it builds cfg. cfg.Folder
// must be the folder saved in the file. The writer is claimed first and the
// file is read again, because the writer of an earlier pane may have saved
// more records since the caller's read. The returned conversation is that
// second read, which is the one to show.
func ResumeLoaded(cfg Config, loaded *convstore.Conversation) (*Conversation, *convstore.Conversation, error) {
	id := loaded.Meta.ID
	if loaded.Meta.Folder != cfg.Folder.Path {
		return nil, nil, fmt.Errorf("resume conversation %s in %s: saved for %s: %w", id, cfg.Folder.Path, loaded.Meta.Folder, ErrFolderMismatch)
	}
	w, err := cfg.Store.Writer(id)
	if err != nil {
		return nil, nil, fmt.Errorf("resume conversation: %w", err)
	}
	loaded, err = cfg.Store.Load(id)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("resume conversation: %w", err), w.Close())
	}
	if loaded.MidTurn {
		// The turn that was running when Snow stopped is over; record it, so
		// the next load does not see it open.
		if err := w.Append(&convstore.Interruption{At: now(cfg), Reason: "Snow stopped"}); err != nil {
			return nil, nil, errors.Join(fmt.Errorf("resume conversation: %w", err), w.Close())
		}
	}
	c := newConversation(cfg, w)
	model, effort, mode := StateOf(loaded)
	c.model, c.effort = model, provider.Effort(effort)
	c.mode.Store(mode)
	c.history = history(loaded.Records)
	c.open = openQuestion(loaded.OpenQuestion)
	for _, r := range loaded.Records {
		if _, ok := r.(*convstore.User); ok {
			c.turns++
		}
	}
	go c.loop()
	return c, loaded, nil
}

// StateOf returns the model ("provider/model"), the effort, and the
// approval mode a loaded conversation ends in: the last of each saved in
// the file, else the ones it started with.
func StateOf(loaded *convstore.Conversation) (model, effort string, mode Mode) {
	model, effort, mode = loaded.Meta.Model, loaded.Meta.Effort, ModeAsk
	for _, r := range loaded.Records {
		switch r := r.(type) {
		case *convstore.ModelSwitch:
			model, effort = r.Model, r.Effort
		case *convstore.Mode:
			if Mode(r.Mode).valid() {
				mode = Mode(r.Mode)
			}
		}
	}
	return model, effort, mode
}

func newConversation(cfg Config, w *convstore.Writer) *Conversation {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.System == "" {
		cfg.System = DefaultSystem(cfg.Folder, cfg.Shell)
	}
	if cfg.Events == nil {
		cfg.Events = func(Event) {}
	}
	c := &Conversation{
		cfg: cfg, id: w.ID(), agent: ID(w.ID()), w: w,
		ev:     newEmitter(cfg.Events),
		cmds:   make(chan any),
		done:   make(chan struct{}),
		grants: map[string]bool{},
	}
	c.mode.Store(ModeAsk)
	return c
}

// ID returns the conversation's ID.
func (c *Conversation) ID() string { return c.id }

// Agent returns the ID of the conversation's main agent.
func (c *Conversation) Agent() ID { return c.agent }

// Send sends a user message. During a turn it waits in the queue and starts
// the next turn when the running one ends.
func (c *Conversation) Send(text string) error { return c.post(sendCmd{text}) }

// SwitchModel makes the next turn use model ("provider/model") with
// effort. A running turn keeps its model. The switch is saved in the
// conversation file before it applies, and a ModelChanged event reports
// it, or why it could not be saved.
func (c *Conversation) SwitchModel(model, effort string) error {
	if _, err := provider.ParseModelName(model); err != nil {
		return fmt.Errorf("switch model: %w: %w", ErrInvalid, err)
	}
	return c.post(switchCmd{&convstore.ModelSwitch{Model: model, Effort: effort}})
}

// Mode returns the approval mode.
func (c *Conversation) Mode() Mode { return c.mode.Load().(Mode) }

// SetMode changes the approval mode, from the next tool call on. The mode
// is saved in the conversation file, so it survives a resume; a
// ModeChanged event reports it, or why it could not be saved.
func (c *Conversation) SetMode(m Mode) error {
	if !m.valid() {
		return fmt.Errorf("set mode %q: %w", m, ErrInvalid)
	}
	return c.post(modeCmd{m})
}

// MaxTitle is the longest title Rename takes, in characters.
const MaxTitle = 120

// Rename gives the conversation a title. Control characters, tabs, and
// line breaks in title become spaces, and runs of spaces become one. A
// blank title, or one longer than MaxTitle characters, gives an error
// wrapping ErrInvalid. The title is saved as a record at the end of the
// conversation file, which is never rewritten, and a TitleChanged event
// reports it, or why it could not be saved.
func (c *Conversation) Rename(title string) error {
	t := convstore.OneLine(title)
	switch n := utf8.RuneCountInString(t); {
	case n == 0:
		return fmt.Errorf("rename: %w: the title is blank", ErrInvalid)
	case n > MaxTitle:
		return fmt.Errorf("rename: %w: the title is %d characters; the limit is %d", ErrInvalid, n, MaxTitle)
	}
	return c.post(renameCmd{t})
}

// Answer answers a waiting approval request. An ID that matches no
// waiting request gives an error wrapping ErrNoApproval.
func (c *Conversation) Answer(a Answer) error {
	switch a.Decision {
	case DecisionYes, DecisionAlways, DecisionNo:
	default:
		return fmt.Errorf("answer with decision %q: %w", a.Decision, ErrInvalid)
	}
	res := make(chan error, 1)
	if err := c.post(answerCmd{a, res}); err != nil {
		return err
	}
	return <-res
}

// Pending returns the approval request that waits for an answer, or nil.
// A client that attaches shows it again.
func (c *Conversation) Pending() *ApprovalRequest {
	res := make(chan *ApprovalRequest, 1)
	if c.post(pendingCmd{res}) != nil {
		return nil
	}
	return <-res
}

// Idle reports whether nothing is running, waiting, or being saved. The
// loop answers in order, so a message sent before the call counts: a caller
// that must not cut a turn short, such as one that replaces the
// conversation, asks here and not in a copy of the state. A closed
// conversation is not idle.
func (c *Conversation) Idle() bool {
	res := make(chan bool, 1)
	if c.post(idleCmd{res}) != nil {
		return false
	}
	return <-res
}

// Retry runs the last turn again, after it failed or was interrupted:
// with no new message when the conversation ends with the user's side,
// and else with the user's last message sent again. It returns ErrBusy
// while a turn runs or messages wait.
func (c *Conversation) Retry() error {
	res := make(chan error, 1)
	if err := c.post(retryCmd{res}); err != nil {
		return err
	}
	return <-res
}

// Interrupt stops the running turn: it cancels the stream and any running
// tool, saves what arrived, and marks the turn interrupted. Queued messages
// stay queued and start the next turn.
func (c *Conversation) Interrupt() error { return c.post(interruptCmd{}) }

// Close stops the running turn, as Interrupt does, waits for it and for
// the mode and model changes still to be saved, delivers the events still
// queued, and closes the file. Queued messages are dropped.
func (c *Conversation) Close() error {
	if err := c.post(closeCmd{}); err != nil {
		return nil
	}
	<-c.done
	c.ev.close()
	return c.w.Close()
}

func (c *Conversation) post(cmd any) error {
	select {
	case c.cmds <- cmd:
		return nil
	case <-c.done:
		return ErrClosed
	}
}

func (c *Conversation) now() time.Time { return c.cfg.Now() }

// now reads cfg's clock before a Conversation exists.
func now(cfg Config) time.Time {
	if cfg.Now == nil {
		return time.Now()
	}
	return cfg.Now()
}

// emit sends an event about the main agent.
func (c *Conversation) emit(b Body) {
	c.ev.emit(Event{Agent: c.agent, Conversation: c.id, At: c.now(), Body: b})
}
