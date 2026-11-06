package api

import (
	"errors"
	"fmt"
)

// RuntimeInfo identifies the external runtime behind a chat. Its session belongs
// to the runtime; Snow never reconstructs model history from the visible chat.
type RuntimeInfo struct {
	Kind        string        `json:"kind"`
	Connection  string        `json:"connection"`
	Auth        string        `json:"auth"`
	Environment string        `json:"environment"`
	Session     string        `json:"session,omitempty"`
	Tasks       []RuntimeTask `json:"tasks,omitempty"`
}

// RuntimeTask is an owned runtime task. IDs and parent IDs determine which work
// a stop affects; folder names never establish ownership.
type RuntimeTask struct {
	ID          string `json:"id"`
	Run         string `json:"run,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Depth       int    `json:"depth,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Description string `json:"description"`
	State       string `json:"state"`
}

// RuntimeRequest asks for a connection operation in a chat's workspace.
// Key travels only in the protected local protocol and is never formatted.
// Create adds a connection only when it works: a Claude plan connection
// signs in first, and an API key connection comes with its Key.
type RuntimeRequest struct {
	RequestID  string `json:"requestId,omitempty"`
	Pane       int    `json:"pane"`
	Agent      string `json:"agent"`
	Action     string `json:"action"`
	Connection string `json:"connection,omitempty"`
	Auth       string `json:"auth,omitempty"`
	Key        string `json:"key,omitempty"`
}

// Connection operations supported by Snow.
const (
	RuntimeList        = "list"
	RuntimeStatus      = "status"
	RuntimeCreate      = "create"
	RuntimeUse         = "use"
	RuntimeLogin       = "login"
	RuntimeCancelLogin = "cancel-login"
	RuntimeSaveKey     = "save-key"
	RuntimeLogout      = "logout"
	// RuntimeRemove signs a connection out and deletes it with its saved
	// key. Saved chats that used it stay.
	RuntimeRemove = "remove"
)

// ErrInvalidRuntime means a connection request is incomplete or unsupported.
var ErrInvalidRuntime = errors.New("api: invalid runtime request")

// Validate checks a connection request before the server reads credentials or
// starts a runtime. Each operation still checks current pane/request ownership.
func (r *RuntimeRequest) Validate() error {
	if r.Pane <= 0 || r.Agent == "" || len(r.Key) > 512 {
		return ErrInvalidRuntime
	}
	switch r.Action {
	case RuntimeList:
	case RuntimeCreate:
		if r.Auth != "subscription" && r.Auth != "api-key" || (r.Auth == "api-key") != (r.Key != "") {
			return ErrInvalidRuntime
		}
	case RuntimeUse, RuntimeLogin, RuntimeCancelLogin, RuntimeLogout, RuntimeStatus, RuntimeRemove:
		if r.Connection == "" {
			return ErrInvalidRuntime
		}
	case RuntimeSaveKey:
		if r.Connection == "" || r.Key == "" {
			return ErrInvalidRuntime
		}
	default:
		return ErrInvalidRuntime
	}
	if r.Key != "" && r.Action != RuntimeSaveKey && r.Action != RuntimeCreate {
		return ErrInvalidRuntime
	}
	return nil
}

// Format hides a key for every fmt verb, including debug formatting.
func (r RuntimeRequest) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprintf(f, "api.RuntimeRequest{Pane:%d, Action:%q, Connection:%q, Key:[hidden]}", r.Pane, r.Action, r.Connection)
}

// RuntimeConnection describes a Snow-owned connection, without credentials.
type RuntimeConnection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Auth        string `json:"auth"`
	Environment string `json:"environment"`
	// Status is one of the RuntimeStatus* values. Unknown is never shown
	// as signed out.
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	// Ready means a runtime started on this connection since its last
	// sign-in change. Signed in alone does not prove the runtime works.
	Ready bool `json:"ready,omitempty"`
}

// RuntimeConnections answers a connection operation. LoginPane names an
// official interactive login running inside Snow, when one was opened.
type RuntimeConnections struct {
	RequestID   string              `json:"requestId,omitempty"`
	Pane        int                 `json:"pane"`
	Agent       string              `json:"agent"`
	Connections []RuntimeConnection `json:"connections"`
	Selected    string              `json:"selected,omitempty"`
	State       string              `json:"state,omitempty"`
	Message     string              `json:"message,omitempty"`
	LoginPane   int                 `json:"loginPane,omitempty"`
	// Ignored names this machine's variables, such as ANTHROPIC_API_KEY,
	// that Snow keeps from every connection. They never choose a route.
	// It is empty for a WSL chat, whose distribution has its own.
	Ignored []string `json:"ignored,omitempty"`
}

// Type implements Message.
func (*RuntimeRequest) Type() MsgType { return TypeRuntimeRequest }
func (*RuntimeRequest) message()      {}

// Type implements Message.
func (*RuntimeConnections) Type() MsgType { return TypeRuntimeConnections }
func (*RuntimeConnections) message()      {}

// The sign-in states of a RuntimeConnection.
const (
	RuntimeStatusUnknown   = "unknown"    // Snow could not learn the state; Reason says why
	RuntimeStatusSignedOut = "signed-out" // no sign-in, or no saved key for an API-key connection
	RuntimeStatusSignedIn  = "signed-in"
	RuntimeStatusKeySaved  = "key-saved"
	RuntimeStatusBlocked   = "blocked" // the organization's managed policy refuses this route
)
