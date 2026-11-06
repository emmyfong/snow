package clauderuntime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/emmyfong/snow/internal/convstore"

	"github.com/emmyfong/snow/pkg/api"
)

func (s *conversationState) input(input api.ChatInput) error {
	if len(s.closing) > 0 || s.saveFailed {
		return ErrClosed
	}
	if input.Kind == api.ChatInputInterrupt {
		s.stop(nil)
		return nil
	}
	if s.authPaused && input.Kind != api.ChatInputRename {
		return fmt.Errorf("%w: connection authentication is being updated", ErrControl)
	}
	if s.stopped != nil || s.snapshot.State == api.ChatStateStopping {
		return errors.New("the Claude run is stopping. Wait for its acknowledgement")
	}
	switch input.Kind {
	case api.ChatInputMessage:
		if !s.snapshot.Running {
			return s.begin(input.Text)
		}
		if len(s.snapshot.Queue) >= api.MaxChatQueue {
			return ErrBackpressure
		}
		queue := append(append([]string(nil), s.snapshot.Queue...), input.Text)
		s.emit(api.ChatEvent{Kind: api.ChatEventQueue, Queue: queue, Paused: s.snapshot.QueuePaused})
		return nil
	case api.ChatInputSendNow:
		if !s.snapshot.Running {
			return s.begin(input.Text)
		}
		s.afterStop = input.Text
		s.stop(nil)
		return nil
	case api.ChatInputResumeQueue:
		if s.snapshot.Running || len(s.snapshot.Queue) == 0 {
			return errors.New("wait for the current run before releasing queued messages")
		}
		s.emit(api.ChatEvent{Kind: api.ChatEventQueue, Queue: s.snapshot.Queue, Paused: false})
		return s.nextQueued()
	case api.ChatInputAnswer, api.ChatInputReply:
		return s.answer(input)
	case api.ChatInputStopTask:
		return s.stopTask(input.ID, input.Run)
	case api.ChatInputRename:
		s.emit(api.ChatEvent{Kind: api.ChatEventTitle, Text: convstore.OneLine(input.Text)})
		return nil
	case api.ChatInputMode:
		if input.Mode == api.ChatModeAsk {
			return nil
		}
		return errors.New("this Claude connection uses Claude's approval rules; Snow cannot broaden them")
	case api.ChatInputModel:
		return s.switchModel(input.Model, input.Effort)
	case api.ChatInputRetry:
		return errors.New("runtime submissions are never retried automatically. Send a new explicit message")
	default:
		return errors.New("this control is not available for Claude Code")
	}
}

// switchModel makes the next start use model and effort. Claude takes them
// only when it starts, so a live Claude restarts at the next message.
func (s *conversationState) switchModel(model, effort string) error {
	if !strings.HasPrefix(model, ModelPrefix) {
		return errors.New("a Claude Code chat uses Claude Code's models. Choose one under Claude Code")
	}
	if s.snapshot.Running {
		return errors.New("wait for this run to finish before you change the model")
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventModel, Model: model, Effort: effort})
	s.restart = s.worker != nil
	return nil
}

func (s *conversationState) begin(text string) error {
	if s.mayHaveRun && s.snapshot.Runtime.Session == "" {
		return errors.New("the runtime did not provide a resumable session ID. Start a new conversation")
	}
	if s.restart && s.worker != nil {
		// Ending the live Claude first; the stop sends text when it is done.
		s.restart, s.afterStop = false, text
		s.stop(nil)
		return nil
	}
	s.restart = false
	if s.worker == nil {
		s.worker = newRunWorker(s.options.Start)
		s.ready, s.results = s.worker.ready, s.worker.results
		s.stopQueries = s.worker.stopQueries
	}
	if err := s.worker.submit(operation{kind: "prompt", text: text, session: s.snapshot.Runtime.Session,
		model: s.snapshot.Model, effort: s.snapshot.Effort}); err != nil {
		return err
	}
	s.failure, s.resultSeen, s.streamed = nil, false, false
	s.streamedThinking, s.sawText = false, false
	s.sessionState = "running"
	s.emit(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: s.snapshot.Turn + 1, Text: text})
	s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStarting})
	return nil
}

func (s *conversationState) nextQueued() error {
	if len(s.snapshot.Queue) == 0 || s.snapshot.QueuePaused {
		return nil
	}
	text := s.snapshot.Queue[0]
	if err := s.begin(text); err != nil {
		return err
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventQueue, Queue: s.snapshot.Queue[1:]})
	return nil
}

func (s *conversationState) operationResult(result operationResult) {
	if s.stopped != nil {
		return
	}
	if result.err != nil {
		if result.operation.kind == "answer" && errors.Is(result.err, ErrRequest) {
			s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther,
				Message: "The runtime cancelled this request before the reply arrived. The reply was not applied."}})
			return
		}
		if result.operation.kind == "stop-task" {
			s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther,
				Message: "Claude did not confirm the child task stopped. The parent remains active; stop the parent to end all owned work."}})
			return
		}
		s.stop(result.err)
		return
	}
	if result.operation.kind == "prompt" && s.snapshot.State == api.ChatStateStarting {
		s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStreaming})
	}
}

func (s *conversationState) stop(cause error) {
	if cause != nil && s.failure == nil {
		s.failure = cause
	}
	if s.stopped != nil {
		return
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStopping})
	s.emit(api.ChatEvent{Kind: api.ChatEventQueue, Queue: s.snapshot.Queue, Paused: true})
	if s.worker == nil {
		s.finishStop(cause)
		return
	}
	s.stopped = s.worker.stop()
}

func (s *conversationState) finishStop(cause error) {
	s.permissions = nil
	for id, task := range s.taskRuns {
		if !terminalTask(task.State) {
			task.State = "stopped"
			s.taskRuns[id] = task
		}
	}
	s.publishRuntime()
	s.emit(api.ChatEvent{Kind: api.ChatEventApproval})
	s.emit(api.ChatEvent{Kind: api.ChatEventQuestion})
	if s.snapshot.Running {
		event := api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: s.snapshot.Turn, Reason: api.ChatEndInterrupted}
		if cause != nil {
			event.Reason = api.ChatEndFailed
			event.Error = &api.ChatError{Code: api.ChatErrOther, Message: fmt.Sprintf("Claude run ended: %v", cause)}
			if errors.Is(cause, ErrSignIn) {
				event.Error.Code, event.Error.Choices = api.ChatErrNoKey, []string{api.ChatChoiceLogin}
			}
			if s.saveFailed {
				event.Error.Code = api.ChatErrSave
			}
		}
		s.emit(event)
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStopped})
	if s.afterStop != "" && len(s.closing) == 0 && !s.saveFailed {
		text := s.afterStop
		s.afterStop = ""
		if err := s.begin(text); err != nil {
			s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther, Message: err.Error()}})
		}
	}
}
