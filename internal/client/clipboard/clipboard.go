package clipboard

// ErrMsg is the message a copy command returns when the copy failed. Only
// Windows can tell: a terminal that is asked to set the clipboard with OSC 52
// does not answer.
type ErrMsg struct{ Err error }

// Error implements error.
func (e ErrMsg) Error() string { return "copy to the clipboard: " + e.Err.Error() }

// Unwrap returns the cause.
func (e ErrMsg) Unwrap() error { return e.Err }
