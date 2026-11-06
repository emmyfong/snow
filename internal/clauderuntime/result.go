package clauderuntime

import "github.com/emmyfong/snow/pkg/api"

func (s *conversationState) result(message runtimeMessage) {
	usage := &api.ChatUsage{Requested: s.snapshot.Model, Input: message.Usage.Input, Output: message.Usage.Output,
		CacheRead: message.Usage.CacheRead, CacheWrite: message.Usage.CacheWrite, Scope: "main-agent", Cumulative: true}
	if message.Cost != nil && *message.Cost >= s.snapshot.Cost {
		usage.Cost, usage.Priced = *message.Cost, true
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventUsage, Usage: usage})
	if !s.sawText && message.Result != "" {
		s.sawText = true
		s.emit(api.ChatEvent{Kind: api.ChatEventText, Text: message.Result})
	}
	s.resultSeen = true
	if message.IsError && s.stopped == nil {
		s.stop(ErrControl)
		return
	}
	s.maybeComplete()
}

func (s *conversationState) maybeComplete() {
	if s.stopped != nil || !s.snapshot.Running || !s.resultSeen || s.sessionState != "idle" || len(s.permissions) > 0 {
		return
	}
	for _, task := range s.taskRuns {
		if !terminalTask(task.State) {
			return
		}
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: s.snapshot.Turn, Reason: api.ChatEndDone})
	s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateIdle})
	if err := s.nextQueued(); err != nil {
		s.emit(api.ChatEvent{Kind: api.ChatEventError, Error: &api.ChatError{Code: api.ChatErrOther, Message: err.Error()}})
	}
}
