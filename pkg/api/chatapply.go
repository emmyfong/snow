package api

import "strings"

// Apply changes s by e, the way the server changed its own copy. Server
// and client fold the same events with it, so they agree on the
// transcript. It ignores an event for another pane and a kind it does not
// know. It never changes a ChatTool in place but swaps in a changed
// copy, so a copy of Entries taken earlier stays safe to read, for
// example by a goroutine that encodes it, while Apply runs.
func (s *ChatSnapshot) Apply(e *ChatEvent) {
	if e.Pane != s.Pane {
		return
	}
	switch e.Kind {
	case ChatEventTurnStarted:
		s.Running, s.Turn = true, e.Turn
		if e.Text != "" {
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryUser, Turn: e.Turn, Text: e.Text})
		}
	case ChatEventText:
		s.appendText(ChatEntryAssistant, e.Text)
	case ChatEventThinking:
		s.appendText(ChatEntryThinking, e.Text)
	case ChatEventToolCall:
		if e.Tool != nil {
			t := *e.Tool
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryTool, Turn: s.Turn, Tool: &t})
		}
	case ChatEventApproval:
		if e.Approval != nil {
			a := *e.Approval
			s.Pending = &a
			s.changeTool(a.ID, func(t *ChatTool) { t.Diff = a.Diff })
		}
	case ChatEventToolResult:
		if e.Tool == nil {
			return
		}
		r := e.Tool
		s.changeTool(r.ID, func(t *ChatTool) { t.Done, t.Result, t.IsError = true, r.Result, r.IsError })
		if s.Pending != nil && s.Pending.ID == e.Tool.ID {
			s.Pending = nil
		}
	case ChatEventTurnEnded:
		s.Running, s.Pending = false, nil
		if e.Reason != ChatEndDone {
			s.cutReply(e.Turn)
		}
		switch e.Reason {
		case ChatEndInterrupted:
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryInterrupted, Turn: e.Turn})
		case ChatEndFailed:
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryError, Turn: e.Turn, Error: e.Error})
		}
	case ChatEventState:
		s.State = e.State
	case ChatEventQueue:
		s.Queue = append([]string(nil), e.Queue...)
	case ChatEventUsage:
		if e.Usage != nil {
			u := *e.Usage
			s.Usage = &u
			if u.Priced {
				s.Cost += u.Cost
			}
			s.noteReported(&u)
		}
	case ChatEventModel:
		if e.Error != nil {
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryError, Error: e.Error})
			return
		}
		s.Model, s.Effort = e.Model, e.Effort
		s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryModel, Text: e.Model})
	case ChatEventMode:
		if e.Error != nil {
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryError, Error: e.Error})
			return
		}
		s.Mode = e.Mode
	case ChatEventTitle:
		if e.Error != nil {
			s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryError, Error: e.Error})
			return
		}
		s.Title = e.Text
	}
}

// appendText adds text to the last entry when it is of kind in the running
// turn, else starts a new entry.
func (s *ChatSnapshot) appendText(kind, text string) {
	if n := len(s.Entries); n > 0 {
		if last := &s.Entries[n-1]; last.Kind == kind && last.Turn == s.Turn {
			last.Text += text
			return
		}
	}
	s.Entries = append(s.Entries, ChatEntry{Kind: kind, Turn: s.Turn, Text: text})
}

// cutReply marks turn's reply as cut off when it is the last entry: the
// model was still writing it.
func (s *ChatSnapshot) cutReply(turn int) {
	if n := len(s.Entries); n > 0 {
		if last := &s.Entries[n-1]; last.Kind == ChatEntryAssistant && last.Turn == turn {
			last.Cut = true
		}
	}
}

// noteReported adds an ChatEntryReported when the vendor reported another
// model than u asked for, once per turn and model.
func (s *ChatSnapshot) noteReported(u *ChatUsage) {
	if reportedMatches(u.Requested, u.Reported) {
		return
	}
	for i := len(s.Entries) - 1; i >= 0 && s.Entries[i].Turn == s.Turn; i-- {
		if e := s.Entries[i]; e.Kind == ChatEntryReported && e.Text == u.Reported && e.Requested == u.Requested {
			return
		}
	}
	s.Entries = append(s.Entries, ChatEntry{Kind: ChatEntryReported, Turn: s.Turn, Text: u.Reported, Requested: u.Requested})
}

// reportedMatches reports whether reported, the model ID a vendor says it
// used, is the model requested ("provider/model"): the same ID, or that ID
// with a dated snapshot suffix such as "-20261001" or "-2026-10-01". An
// empty reported ID matches, as the vendor did not say.
func reportedMatches(requested, reported string) bool {
	if reported == "" {
		return true
	}
	_, model, ok := strings.Cut(requested, "/")
	if !ok {
		model = requested
	}
	if reported == model {
		return true
	}
	rest, ok := strings.CutPrefix(reported, model+"-")
	if !ok || len(rest) < 8 {
		return false
	}
	for _, r := range rest {
		if (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// changeTool replaces the call with id, the latest first, by a copy that
// change has changed. It does nothing when no call has id.
func (s *ChatSnapshot) changeTool(id string, change func(*ChatTool)) {
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if t := s.Entries[i].Tool; t != nil && t.ID == id {
			c := *t
			change(&c)
			s.Entries[i].Tool = &c
			return
		}
	}
}
