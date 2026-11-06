package clauderuntime

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/emmyfong/snow/pkg/api"
)

type pendingPermission struct {
	id         string
	permission Permission
	questions  []api.ChatQuestion
	labels     []map[string]string
	index      int
	answers    map[string]string
	original   map[string]json.RawMessage
}

func (s *conversationState) permission(frame Frame) error {
	if len(s.permissions) >= 16 {
		return ErrBackpressure
	}
	var wire struct {
		Request Permission `json:"request"`
	}
	if json.Unmarshal(frame.Raw, &wire) != nil {
		return ErrProtocol
	}
	pending := pendingPermission{id: frame.RequestID, permission: wire.Request}
	if wire.Request.Tool == "AskUserQuestion" {
		if err := pending.parseQuestions(); err != nil {
			return err
		}
	}
	s.permissions = append(s.permissions, pending)
	if len(s.permissions) == 1 {
		s.showPermission()
	}
	return nil
}

func (p *pendingPermission) parseQuestions() error {
	var input struct {
		Questions []struct {
			Question string `json:"question"`
			Multi    bool   `json:"multiSelect"`
			Options  []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(p.permission.Input, &input) != nil || len(input.Questions) == 0 || len(input.Questions) > 4 {
		return fmt.Errorf("%w: unsupported question format", ErrProtocol)
	}
	if err := json.Unmarshal(p.permission.Input, &p.original); err != nil {
		return ErrProtocol
	}
	p.answers = map[string]string{}
	for i, question := range input.Questions {
		if question.Question == "" || len(question.Question) > api.MaxChatMessage || len(question.Options) > api.MaxChatPicked {
			return fmt.Errorf("%w: question exceeds supported limits", ErrProtocol)
		}
		for _, previous := range p.questions {
			if previous.Text == question.Question {
				return fmt.Errorf("%w: duplicate question identity", ErrProtocol)
			}
		}
		q := api.ChatQuestion{ID: fmt.Sprintf("%s:%d", p.id, i), Text: question.Question, Multi: question.Multi}
		labels := map[string]string{}
		for n, option := range question.Options {
			label, description := option.Label, option.Description
			if label == "" || len(label) > 4096 || len(description) > 4096 {
				return ErrProtocol
			}
			if utf8.RuneCountInString(label) > api.MaxChatLabel {
				label = string([]rune(label)[:api.MaxChatLabel-10]) + fmt.Sprintf("… (%d)", n+1)
				description = option.Label + "\n" + description
			}
			if _, duplicate := labels[label]; duplicate {
				return fmt.Errorf("%w: duplicate question option", ErrProtocol)
			}
			labels[label] = option.Label
			q.Options = append(q.Options, api.ChatOption{Label: label, Description: description})
		}
		p.questions, p.labels = append(p.questions, q), append(p.labels, labels)
	}
	return nil
}

func (s *conversationState) showPermission() {
	if len(s.permissions) == 0 {
		s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateTool})
		return
	}
	pending := &s.permissions[0]
	if len(pending.questions) > 0 {
		question := pending.questions[pending.index]
		s.emit(api.ChatEvent{Kind: api.ChatEventQuestion, Question: &question})
	} else {
		s.emit(api.ChatEvent{Kind: api.ChatEventApproval, Approval: &api.ChatApproval{
			ID: pending.id, Tool: pending.permission.Tool, Input: pending.permission.Input}})
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateApproval})
}

func (s *conversationState) answer(input api.ChatInput) error {
	if len(s.permissions) == 0 || s.worker == nil {
		return ErrRequest
	}
	pending := &s.permissions[0]
	op := operation{kind: "answer", id: pending.id, allow: input.Decision == api.ChatDecisionYes, text: input.Text}
	var savedAnswer *api.ChatEvent
	if len(pending.questions) > 0 {
		question := pending.questions[pending.index]
		if input.Kind != api.ChatInputReply || input.ID != question.ID || !question.Multi && len(input.Picked) > 1 {
			return ErrRequest
		}
		var picked []string
		for _, label := range input.Picked {
			original, exists := pending.labels[pending.index][label]
			if !exists || slices.Contains(picked, original) {
				return ErrRequest
			}
			picked = append(picked, original)
		}
		answer := input.Text
		if len(picked) > 0 {
			answer = strings.Join(picked, ", ")
		}
		pending.answers[question.Text] = answer
		savedAnswer = &api.ChatEvent{Kind: api.ChatEventAnswered, Question: &api.ChatQuestion{ID: question.ID},
			Text: question.Text + "\n" + answer}
		if pending.index+1 < len(pending.questions) {
			pending.index++
			s.emit(*savedAnswer)
			s.showPermission()
			return nil
		}
		answers, err := json.Marshal(pending.answers)
		if err != nil {
			return err
		}
		pending.original["answers"] = answers
		updated, err := json.Marshal(pending.original)
		if err != nil {
			return err
		}
		op.allow, op.input = true, updated
	} else {
		if input.Kind != api.ChatInputAnswer || input.ID != pending.id {
			return ErrRequest
		}
		if input.Decision == api.ChatDecisionAlways {
			return fmt.Errorf("%w: Claude approvals support Yes or No here; an Always grant is not available", ErrControl)
		}
	}
	if err := s.worker.submit(op); err != nil {
		return err
	}
	s.emit(api.ChatEvent{Kind: api.ChatEventApproval})
	if savedAnswer != nil {
		s.emit(*savedAnswer)
	} else if s.snapshot.Question != nil {
		s.emit(api.ChatEvent{Kind: api.ChatEventAnswered, Question: &api.ChatQuestion{ID: s.snapshot.Question.ID}})
	}
	s.permissions = s.permissions[1:]
	s.showPermission()
	return nil
}

func (s *conversationState) cancelPermission(id string) {
	for i, pending := range s.permissions {
		if pending.id != id {
			continue
		}
		s.permissions = append(s.permissions[:i:i], s.permissions[i+1:]...)
		if i == 0 {
			s.emit(api.ChatEvent{Kind: api.ChatEventApproval})
			s.emit(api.ChatEvent{Kind: api.ChatEventQuestion})
			s.showPermission()
		}
		return
	}
}
