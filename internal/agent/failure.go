package agent

import (
	"errors"
	"time"

	"github.com/emmyfong/snow/internal/provider"
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

// failure describes a provider error for the user, with what the user can
// do about it. The agent itself never retries and never changes the model.
func failure(err error) *Failure {
	f := &Failure{Kind: FailOther, Message: err.Error(), Choices: []Choice{ChoiceRetry, ChoicePickModel}}
	var pe *provider.Error
	if !errors.As(err, &pe) {
		return f
	}
	switch pe.Kind {
	case provider.KindBadKey:
		f.Kind, f.Choices = FailBadKey, []Choice{ChoiceLogin, ChoicePickModel}
	case provider.KindNoKey:
		f.Kind, f.Choices = FailNoKey, []Choice{ChoiceLogin, ChoicePickModel}
	case provider.KindModelNotFound:
		f.Kind, f.Choices = FailModelNotFound, []Choice{ChoicePickModel}
	case provider.KindRateLimited:
		f.Kind, f.RetryAfter, f.Choices = FailRateLimited, pe.RetryAfter, []Choice{ChoiceWait, ChoiceRetry, ChoicePickModel}
	case provider.KindOverloaded:
		f.Kind, f.Choices = FailOverloaded, []Choice{ChoiceRetry, ChoiceWait, ChoicePickModel}
	case provider.KindContextTooLong:
		f.Kind, f.Choices = FailContextTooLong, []Choice{ChoicePickModel}
	case provider.KindNetwork:
		f.Kind, f.Choices = FailNetwork, []Choice{ChoiceRetry}
	}
	return f
}
