package agent

import (
	"time"
)

// FailureKind says why a turn failed, so the user gets the right choices.
type FailureKind string

// The kinds of failure.
const (
	FailSave           FailureKind = "save"             // the conversation file could not be written
	FailBadKey         FailureKind = "bad_key"          // the provider rejected the key
	FailNoKey          FailureKind = "no_key"           // no key is saved for the provider
	FailModelNotFound  FailureKind = "model_not_found"  // the model does not exist or was retired
	FailRateLimited    FailureKind = "rate_limited"     // see Failure.RetryAfter
	FailOverloaded     FailureKind = "overloaded"       // the provider is busy
	FailContextTooLong FailureKind = "context_too_long" // the conversation does not fit the model
	FailNetwork        FailureKind = "network"          // the provider could not be reached
	FailOther          FailureKind = "other"
)

// Choice is something the user can do after a failure. The agent never
// does any of them by itself.
type Choice string

// The choices after a failure.
const (
	ChoiceRetry     Choice = "retry"      // send the turn again
	ChoicePickModel Choice = "pick_model" // choose another model
	ChoiceWait      Choice = "wait"       // wait for RetryAfter, then retry
	ChoiceLogin     Choice = "login"      // save a key
)

// Failure is why a turn stopped.
type Failure struct {
	Kind       FailureKind
	Message    string
	RetryAfter time.Duration // for FailRateLimited, when the provider says
	Choices    []Choice
}

// failure describes a provider error for the user.
func failure(err error) *Failure {
	return &Failure{Kind: FailOther, Message: err.Error(), Choices: []Choice{ChoiceRetry}}
}
