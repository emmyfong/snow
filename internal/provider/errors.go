package provider

import (
	"fmt"
	"time"
)

// Kind is the kind of a provider failure.
type Kind int

// The kinds of failure a caller can act on.
const (
	KindBadKey         Kind = iota + 1 // the key was rejected
	KindNoKey                          // no key could be read
	KindModelNotFound                  // the model does not exist or was retired
	KindRateLimited                    // too many requests; see RetryAfter
	KindOverloaded                     // the vendor is busy
	KindContextTooLong                 // the request does not fit the model
	KindNetwork                        // the vendor could not be reached
	KindOther                          // any other refusal
)

// String says what went wrong in a few words.
func (k Kind) String() string {
	switch k {
	case KindBadKey:
		return "the API key was rejected"
	case KindNoKey:
		return "no API key"
	case KindModelNotFound:
		return "model not found"
	case KindRateLimited:
		return "rate limited"
	case KindOverloaded:
		return "the provider is overloaded"
	case KindContextTooLong:
		return "the conversation is too long for the model"
	case KindNetwork:
		return "network error"
	default:
		return "provider error"
	}
}

// Error is a failure from a provider. Match it with errors.As and read
// Kind. Err holds the cause, which never contains the key.
type Error struct {
	Kind       Kind
	RetryAfter time.Duration // for KindRateLimited, when the vendor says
	Err        error
}

// Error gives the kind, the retry time when known, and the cause.
func (e *Error) Error() string {
	msg := e.Kind.String()
	if e.Kind == KindRateLimited && e.RetryAfter > 0 {
		msg += fmt.Sprintf("; retry in %s", e.RetryAfter.Round(time.Second))
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap returns the cause, so errors.Is and errors.As reach it.
func (e *Error) Unwrap() error { return e.Err }
