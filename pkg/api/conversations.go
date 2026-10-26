package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidConversationList is returned by ConversationListRequest.Validate.
var ErrInvalidConversationList = errors.New("api: invalid conversation list request")

// ConversationListRequest asks for the past conversations of a chat pane:
// those of its folder, or of every folder with All. The server answers
// with a ConversationList.
type ConversationListRequest struct {
	Pane int  `json:"pane"`
	All  bool `json:"all,omitempty"`
}

// Validate checks that r names a pane.
func (r *ConversationListRequest) Validate() error {
	if r.Pane <= 0 {
		return fmt.Errorf("%w: no pane", ErrInvalidConversationList)
	}
	return nil
}

// ConversationList is the server's answer to ConversationListRequest,
// newest first. Pane and All repeat the request's, so a client can drop an
// answer to a question it no longer asks.
type ConversationList struct {
	Pane  int                `json:"pane"`
	All   bool               `json:"all,omitempty"`
	Items []ConversationInfo `json:"items"`
}

// ConversationInfo describes one past conversation.
type ConversationInfo struct {
	ID           string    `json:"id"`
	Title        string    `json:"title,omitempty"` // the user's, else the start of the first message
	Folder       string    `json:"folder,omitempty"`
	Model        string    `json:"model,omitempty"` // "provider/model", the latest in the file
	LastActivity time.Time `json:"lastActivity"`
	Turns        int       `json:"turns,omitempty"` // user messages
	// Open says a pane shows the conversation now. Resuming it moves focus
	// to that pane.
	Open bool `json:"open,omitempty"`
	// Err is why the file cannot be read; the file is kept and can be
	// listed but not resumed.
	Err string `json:"err,omitempty"`
}

// MarshalJSON writes Items as [] when it is empty.
func (l ConversationList) MarshalJSON() ([]byte, error) {
	type wire ConversationList
	if l.Items == nil {
		l.Items = []ConversationInfo{}
	}
	return json.Marshal(wire(l))
}

// Type implements Message.
func (*ConversationListRequest) Type() MsgType { return TypeConversationListRequest }
func (*ConversationListRequest) message()      {}

// Type implements Message.
func (*ConversationList) Type() MsgType { return TypeConversationList }
func (*ConversationList) message()      {}
