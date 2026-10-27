package api

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ChatQuestion is a question the agent asks the user. The user picks
// among Options, or types another answer. Options never holds a choice for
// a typed answer: a client offers one itself and sends the text in
// ChatInput.Text.
type ChatQuestion struct {
	ID      string       `json:"id"` // the ask tool call's ID
	Text    string       `json:"text,omitempty"`
	Options []ChatOption `json:"options,omitempty"`
	Multi   bool         `json:"multi,omitempty"` // more than one option may be picked
}

// ChatOption is one answer a ChatQuestion offers.
type ChatOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"` // shown while the option is highlighted
}

// ChatTask is one item of the agent's task list.
type ChatTask struct {
	Text  string `json:"text"`
	State string `json:"state"` // one of the ChatTask* values; show another as pending
}

// States of a ChatTask.
const (
	ChatTaskPending    = "pending"
	ChatTaskInProgress = "in_progress"
	ChatTaskDone       = "done"
)

// Bounds on a ChatInputReply, which match what a question can offer.
// Keep them in step with the ask tool's limits in internal/tool/ask.go.
const (
	MaxChatPicked = 9  // options picked
	MaxChatLabel  = 60 // characters in a label
)

// validateReply checks a ChatInputReply.
func (in *ChatInput) validateReply() error {
	switch {
	case in.ID == "":
		return fmt.Errorf("%w: reply names no question", ErrInvalidChatInput)
	case len(in.Picked) > 0 && in.Text != "":
		return fmt.Errorf("%w: a reply picks options or types an answer, not both", ErrInvalidChatInput)
	case len(in.Picked) == 0 && strings.TrimSpace(in.Text) == "":
		return fmt.Errorf("%w: empty reply", ErrInvalidChatInput)
	case len(in.Picked) > MaxChatPicked:
		return fmt.Errorf("%w: over %d options picked", ErrInvalidChatInput, MaxChatPicked)
	}
	for _, l := range in.Picked {
		if l == "" || utf8.RuneCountInString(l) > MaxChatLabel {
			return fmt.Errorf("%w: a picked label is empty or over %d characters", ErrInvalidChatInput, MaxChatLabel)
		}
	}
	return nil
}
