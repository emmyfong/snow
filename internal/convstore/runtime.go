package convstore

import (
	"encoding/json"
	"time"
)

// TypeRuntimeEvent is a display event from an external coding runtime.
const TypeRuntimeEvent Type = "runtime_event"

// RuntimeMeta binds a Snow conversation to one external connection. Its model
// history stays in the runtime's own session store, not Assistant.Blocks.
type RuntimeMeta struct {
	Kind       string `json:"kind"`
	Connection string `json:"connection"`
}

// RuntimeEvent retains a display event and the latest backend session identity.
// Display is owned by the runtime integration, not a provider message format.
// User is set only when an event begins a user turn, for the history index.
type RuntimeEvent struct {
	At      time.Time       `json:"at"`
	Session string          `json:"session,omitempty"`
	User    string          `json:"user,omitempty"`
	Title   string          `json:"title,omitempty"`
	Model   string          `json:"model,omitempty"`
	Display json.RawMessage `json:"display"`
}

// Type implements Record.
func (*RuntimeEvent) Type() Type { return TypeRuntimeEvent }
