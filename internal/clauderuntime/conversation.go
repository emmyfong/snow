package clauderuntime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// ConversationOptions binds a runtime conversation to its journal and workspace.
// Start must honor cancellation and clean up an unsuccessful initialization.
type ConversationOptions struct {
	Writer   *convstore.Writer
	Snapshot api.ChatSnapshot
	Start    func(context.Context, string) (*Client, error)
	Events   func(api.ChatEvent)
}

// Conversation owns a Claude chat's queue and controls. Claude owns model
// history and tool execution; Snow stores only its display and session binding.
type Conversation struct {
	id, agent string
	commands  chan conversationCommand
	done      chan struct{}
	final     api.ChatSnapshot // read after done closes
	finalErr  error            // read after done closes
}

type conversationCommand struct {
	input      api.ChatInput
	reply      chan error
	snap       chan api.ChatSnapshot
	close      bool
	authPause  bool
	authResume bool
}

// NewConversation starts a paused controller. It launches no runtime or model
// request until an explicit message. Restored approvals cannot authorize work in
// a new process; the runtime must ask again with a current request identity.
func NewConversation(options ConversationOptions) (*Conversation, error) {
	if options.Writer == nil || options.Start == nil || options.Snapshot.Runtime == nil {
		return nil, ErrProfile
	}
	if options.Snapshot.Running {
		return nil, fmt.Errorf("%w: the saved runtime run has no verified end; recovery is not available", ErrControl)
	}
	c := &Conversation{id: options.Writer.ID(), agent: "claude-" + rand.Text(),
		commands: make(chan conversationCommand), done: make(chan struct{})}
	snapshot := options.Snapshot
	info := *snapshot.Runtime
	info.Tasks = nil
	snapshot.Runtime = &info
	snapshot.Agent, snapshot.Conversation = c.agent, c.id
	snapshot.Running, snapshot.Pending, snapshot.Question = false, nil, nil
	if snapshot.State != api.ChatStateStopped {
		snapshot.State = api.ChatStateIdle
	}
	state := &conversationState{conversation: c, options: options, snapshot: snapshot,
		recorder: newRecorder(options.Writer), taskRuns: map[string]api.RuntimeTask{}, mayHaveRun: snapshot.Turn > 0}
	go state.loop()
	return c, nil
}

// ID is Snow's persisted conversation identity.
func (c *Conversation) ID() string { return c.id }

// Agent is this controller generation, which changes on explicit resume.
func (c *Conversation) Agent() string { return c.agent }

// Input validates a current chat control and queues its I/O without blocking
// the controller. A later delivery failure is reported as an event, never retried.
func (c *Conversation) Input(input api.ChatInput) error {
	if input.Agent != "" && input.Agent != c.agent {
		return ErrRequest
	}
	input.Agent, input.Pane = c.agent, 1
	if err := input.Validate(); err != nil {
		return err
	}
	reply := make(chan error, 1)
	select {
	case c.commands <- conversationCommand{input: input, reply: reply}:
	case <-c.done:
		return ErrClosed
	}
	select {
	case err := <-reply:
		return err
	case <-c.done:
		return ErrClosed
	}
}

// Snapshot returns control state. The transcript stays in the server and journal
// so streaming text is not repeatedly concatenated in a second full history.
func (c *Conversation) Snapshot() api.ChatSnapshot {
	reply := make(chan api.ChatSnapshot, 1)
	select {
	case c.commands <- conversationCommand{snap: reply}:
		return <-reply
	case <-c.done:
		return c.final
	}
}

// Close stops the owned runtime, drains the journal, and releases the saved
// conversation. Stop failures leave the controller available and visible.
func (c *Conversation) Close() error {
	reply := make(chan error, 1)
	select {
	case c.commands <- conversationCommand{close: true, reply: reply}:
	case <-c.done:
		return c.finalErr
	}
	select {
	case err := <-reply:
		return err
	case <-c.done:
		select {
		case err := <-reply:
			return err
		default:
			return c.finalErr
		}
	}
}

type conversationState struct {
	conversation     *Conversation
	options          ConversationOptions
	snapshot         api.ChatSnapshot
	recorder         *recorder
	worker           *runWorker
	frames           <-chan Frame
	ready            <-chan *Client
	results          <-chan operationResult
	stopQueries      <-chan chan []stopTaskRef
	stopped          <-chan error
	closing          []chan error
	authReplies      []chan error
	authPaused       bool
	failure          error
	saveFailed       bool
	permissions      []pendingPermission
	taskRuns         map[string]api.RuntimeTask
	resultSeen       bool
	sessionState     string
	streamed         bool
	streamedThinking bool
	mayHaveRun       bool
	sawText          bool
	afterStop        string
	stopUnconfirmed  bool
}

func (s *conversationState) loop() {
	defer func() {
		s.conversation.final = s.snapshot
		close(s.conversation.done)
	}()
	for {
		select {
		case command := <-s.conversation.commands:
			if command.authPause {
				if s.snapshot.Running || s.stopped != nil || len(s.closing) > 0 {
					command.reply <- fmt.Errorf("%w: stop chats using this connection before changing authentication", ErrControl)
					continue
				}
				s.authPaused = true
				s.authReplies = append(s.authReplies, command.reply)
				s.stop(nil)
				if s.worker == nil {
					s.finishAuthPause(nil)
				}
				continue
			}
			if command.authResume {
				s.authPaused = false
				command.reply <- nil
				continue
			}
			if command.snap != nil {
				command.snap <- s.snapshot
				continue
			}
			if command.close {
				s.closing = append(s.closing, command.reply)
				if s.stopUnconfirmed {
					s.conversation.finalErr = errors.Join(ErrStop, s.recorder.close())
					for _, reply := range s.closing {
						reply <- s.conversation.finalErr
					}
					return
				}
				s.stop(nil)
			} else {
				command.reply <- s.input(command.input)
			}
		case client := <-s.ready:
			s.ready, s.frames = nil, client.Frames()
			s.mayHaveRun = true
		case result := <-s.results:
			s.operationResult(result)
		case reply := <-s.stopQueries:
			s.drainRuntimeFrames()
			var tasks []stopTaskRef
			for _, task := range s.taskRuns {
				if !terminalTask(task.State) {
					tasks = append(tasks, stopTaskRef{id: task.ID, run: task.Run})
				}
			}
			reply <- tasks
		case frame, ok := <-s.frames:
			if !ok {
				s.frames = nil
				if s.stopped == nil {
					s.stop(ErrClosed)
				}
			} else if err := s.frame(frame); err != nil {
				s.stop(err)
			}
		case err := <-s.recorder.failed:
			s.saveFailed = true
			s.stop(fmt.Errorf("save runtime conversation: %w", err))
		case err := <-s.stopped:
			s.drainRuntimeFrames()
			for _, task := range s.taskRuns {
				if !terminalTask(task.State) {
					err = errors.Join(err, ErrStop)
				}
			}
			s.stopped, s.frames, s.ready, s.results, s.stopQueries = nil, nil, nil, nil, nil
			if errors.Is(err, ErrStop) {
				s.stopUnconfirmed = true
				s.finishAuthPause(err)
				s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther,
					Message: "Claude did not confirm all owned work stopped. Check its tasks in the selected environment before closing."}})
				for _, reply := range s.closing {
					reply <- err
				}
				if len(s.closing) > 0 {
					s.conversation.finalErr = errors.Join(err, s.recorder.close())
					return
				}
				s.closing = nil
				continue
			}
			s.worker = nil
			if err != nil && s.failure == nil {
				s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther,
					Message: "Claude did not acknowledge the interrupt; process shutdown was confirmed."}})
			}
			s.finishStop(s.failure)
			s.finishAuthPause(nil)
		}
		if s.failure != nil && s.stopped == nil && s.worker != nil {
			s.stop(s.failure)
		}
		if len(s.closing) > 0 && s.worker == nil {
			err := s.recorder.close()
			s.conversation.finalErr = err
			for _, reply := range s.closing {
				reply <- err
			}
			return
		}
	}
}

func (s *conversationState) emit(event api.ChatEvent) {
	event.Pane, event.Agent = s.snapshot.Pane, s.conversation.agent
	s.snapshot.Apply(&event)
	s.snapshot.Entries = nil
	if !s.saveFailed {
		if err := s.recorder.append(event, s.snapshot.Runtime.Session); err != nil {
			s.saveFailed, s.failure = true, err
		}
	}
	if s.options.Events != nil {
		s.options.Events(event)
	}
}
