// Package agent runs Snow's own agent: a conversation that sends turns to
// a provider, runs the tools the model calls, saves every record to its
// conversation file before anything depends on it, and reports what it
// does as events.
package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

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
	// Shell is the profile saved with the conversation; the run tool uses
	// it.
	Shell  shell.Profile
	System string // the system prompt
	// Events receives every event, in order, from one goroutine of the
	// Conversation's own. A slow receiver delays later events, never the
	// turn.
	Events func(Event)
	Now    func() time.Time // the clock; nil means time.Now
}

// errClosed is the cause of a call to a closed Conversation.
var errClosed = errors.New("conversation is closed")

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

	mode atomic.Value // Mode; set by the loop, read by the turn
}

// Commands posted to the loop.
type (
	sendCmd   struct{ text string }
	switchCmd struct{ sw *convstore.ModelSwitch }
	modeCmd   struct{ mode Mode }
	waitCmd   struct {
		req   *ApprovalRequest
		reply chan<- Answer
	}
	answerCmd struct {
		a   Answer
		res chan<- error
	}
	pendingCmd   struct{ res chan<- *ApprovalRequest }
	interruptCmd struct{}
	closeCmd     struct{}
)

// Start creates a new conversation with meta, saves its meta record, and
// starts its goroutine. Meta.Folder is taken from cfg.Folder.
func Start(cfg Config, meta convstore.Meta) (*Conversation, error) {
	meta.Folder = cfg.Folder.Path
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
// show. The conversation goes on with the model saved in its file.
func Resume(cfg Config, id string) (*Conversation, *convstore.Conversation, error) {
	loaded, err := cfg.Store.Load(id)
	if err != nil {
		return nil, nil, fmt.Errorf("resume conversation: %w", err)
	}
	w, err := cfg.Store.Writer(id)
	if err != nil {
		return nil, nil, fmt.Errorf("resume conversation: %w", err)
	}
	c := newConversation(cfg, w)
	c.model, c.effort = loaded.Meta.Model, provider.Effort(loaded.Meta.Effort)
	for _, r := range loaded.Records {
		if s, ok := r.(*convstore.ModelSwitch); ok {
			c.model, c.effort = s.Model, provider.Effort(s.Effort)
		}
	}
	c.history = history(loaded.Records)
	for _, r := range loaded.Records {
		if m, ok := r.(*convstore.Mode); ok && Mode(m.Mode).valid() {
			c.mode.Store(Mode(m.Mode))
		}
	}
	if loaded.MidTurn {
		// The turn that was running when Snow stopped is over; record it, so
		// the next load does not see it open.
		if err := w.Append(&convstore.Interruption{At: c.now(), Reason: "Snow stopped"}); err != nil {
			return nil, nil, errors.Join(fmt.Errorf("resume conversation: %w", err), w.Close())
		}
	}
	go c.loop()
	return c, loaded, nil
}

func newConversation(cfg Config, w *convstore.Writer) *Conversation {
	if cfg.Now == nil {
		cfg.Now = time.Now
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
		return fmt.Errorf("switch model: %w", err)
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
		return fmt.Errorf("set mode: unknown mode %q", m)
	}
	return c.post(modeCmd{m})
}

// Answer answers a waiting approval request. An ID that matches no
// waiting request gives an error wrapping ErrNoApproval.
func (c *Conversation) Answer(a Answer) error {
	switch a.Decision {
	case DecisionYes, DecisionAlways, DecisionNo:
	default:
		return fmt.Errorf("answer: unknown decision %q", a.Decision)
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

// Interrupt stops the running turn: it cancels the stream and any running
// tool, saves what arrived, and marks the turn interrupted. Queued messages
// stay queued and start the next turn.
func (c *Conversation) Interrupt() error { return c.post(interruptCmd{}) }

// Close stops the running turn, as Interrupt does, waits for it, delivers
// the events still queued, and closes the file.
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
		return errClosed
	}
}

func (c *Conversation) now() time.Time { return c.cfg.Now() }

// emit sends an event about the main agent.
func (c *Conversation) emit(b Body) {
	c.ev.emit(Event{Agent: c.agent, Conversation: c.id, At: c.now(), Body: b})
}

// loop owns the queue and starts turns one at a time.
func (c *Conversation) loop() {
	defer close(c.done)
	var (
		queue   []string
		pending *convstore.ModelSwitch // applies when the running turn ends
		waiting *waitCmd               // the approval the turn waits for
		cancel  context.CancelFunc     // of the running turn; nil when idle
		ended   = make(chan struct{})
		turn    int
		closing bool
	)
	start := func(text string) {
		turn++
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go func(n int) {
			c.runTurn(ctx, n, text)
			ended <- struct{}{}
		}(turn)
	}
	for {
		select {
		case cmd := <-c.cmds:
			switch cmd := cmd.(type) {
			case sendCmd:
				if cancel == nil {
					start(cmd.text)
				} else {
					queue = append(queue, cmd.text)
					c.emit(&QueueChanged{Messages: append([]string(nil), queue...)})
				}
			case switchCmd:
				if cancel == nil {
					c.applySwitch(cmd.sw)
				} else {
					pending = cmd.sw
				}
			case modeCmd:
				c.applyMode(cmd.mode)
			case waitCmd:
				waiting = &cmd
			case answerCmd:
				if waiting == nil || waiting.req.ID != cmd.a.ID {
					cmd.res <- fmt.Errorf("answer %q: %w", cmd.a.ID, ErrNoApproval)
					continue
				}
				waiting.reply <- cmd.a
				waiting = nil
				cmd.res <- nil
			case pendingCmd:
				if waiting != nil {
					cmd.res <- waiting.req
				} else {
					cmd.res <- nil
				}
			case interruptCmd:
				if cancel != nil {
					cancel()
				}
			case closeCmd:
				if cancel == nil {
					return
				}
				closing = true
				cancel()
			}
		case <-ended:
			cancel()
			cancel = nil
			waiting = nil
			if closing {
				return
			}
			if pending != nil {
				c.applySwitch(pending)
				pending = nil
			}
			if len(queue) == 0 {
				c.emit(&StateChanged{State: StateIdle})
				continue
			}
			next := queue[0]
			queue = queue[1:]
			c.emit(&QueueChanged{Messages: append([]string(nil), queue...)})
			start(next)
		}
	}
}

// applySwitch saves sw and makes it the model of the next turn. It runs
// only between turns, so no turn reads the model while it changes.
func (c *Conversation) applySwitch(sw *convstore.ModelSwitch) {
	sw.At = c.now()
	if err := c.w.Append(sw); err != nil {
		c.emit(&ModelChanged{Model: c.model, Effort: string(c.effort), Failure: &Failure{Kind: FailSave, Message: err.Error()}})
		return
	}
	c.model, c.effort = sw.Model, provider.Effort(sw.Effort)
	c.emit(&ModelChanged{Model: c.model, Effort: sw.Effort})
}

// applyMode saves m and puts it in force. The file write may race a turn's
// writes; the Writer orders them.
func (c *Conversation) applyMode(m Mode) {
	if err := c.w.Append(&convstore.Mode{At: c.now(), Mode: string(m)}); err != nil {
		c.emit(&ModeChanged{Mode: c.Mode(), Failure: &Failure{Kind: FailSave, Message: err.Error()}})
		return
	}
	c.mode.Store(m)
	c.emit(&ModeChanged{Mode: m})
}
