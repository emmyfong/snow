// Package provider is Snow's boundary to model vendors. A Provider lists its
// models and streams a reply to a Request as Events. Each vendor is its own
// sub-package, so adding one changes nothing else. Failures come back as
// *Error, whose Kind tells the caller what to offer: a new key, another
// model, or a wait.
package provider

import (
	"context"
	"encoding/json"
)

// Provider talks to one model vendor.
type Provider interface {
	// Name is the vendor's part of a model name, for example "anthropic".
	Name() string
	// Models lists the models the key can use.
	Models(ctx context.Context) ([]Model, error)
	// Stream sends req and returns its reply as a Stream. Canceling ctx
	// ends the stream: its next Next returns the context's error.
	Stream(ctx context.Context, req Request) (Stream, error)
}

// Stream is one reply, read event by event. One goroutine calls Next; to
// stop a Next that waits, cancel the context given to Provider.Stream.
type Stream interface {
	// Next returns the next event, io.EOF after the last one, or an error
	// that ends the stream.
	Next() (Event, error)
	// Close stops the stream and frees its connection; Next then returns
	// io.EOF. It is safe to call more than once and after the end.
	Close() error
}

// KeySource gives a provider its API key when it needs one, so a key saved
// or removed while Snow runs takes effect on the next request.
// internal/credential satisfies it. The key is a plain string because the
// vendor's client needs one; a provider never puts it in an error or a log.
type KeySource interface {
	APIKey(provider string) (string, error)
}

// Model is a model a provider offers.
type Model struct {
	ID            string // the vendor's ID, for example "claude-opus-5-5"
	DisplayName   string
	ContextWindow int // in tokens; 0 when the vendor does not say
	// Efforts lists the effort levels the model takes, in AllEfforts order.
	// Nil means it takes none.
	Efforts []Effort
}

// Effort is how hard the model thinks before it answers. Not every model
// takes every level; a provider leaves out a level the model does not take.
type Effort string

// The efforts a Request may ask for. An empty Effort leaves the vendor's
// default.
const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
	EffortMax    Effort = "max"
)

// AllEfforts lists every effort level, from the least to the most thinking.
var AllEfforts = [...]Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}

// Request is one turn sent to a model.
type Request struct {
	Model     string // the vendor's model ID, without the provider part
	Effort    Effort
	System    string
	Messages  []Message
	Tools     []ToolDef
	MaxTokens int // 0 lets the provider choose
}

// Role says who wrote a Message.
type Role string

// The roles of a conversation.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of the conversation sent with a Request.
type Message struct {
	Role    Role
	Content []Block
}

// ToolDef describes a tool the model may call.
type ToolDef struct {
	Name        string
	Description string
	Schema      json.RawMessage // a JSON Schema object for the input
}
