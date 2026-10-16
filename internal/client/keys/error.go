package keys

import (
	"errors"
	"fmt"
)

// Kinds of failure, for errors.Is.
var (
	// ErrBadKey means a key name that Snow cannot read.
	ErrBadKey = errors.New("unknown key")
	// ErrUnknownAction means an action name that does not exist.
	ErrUnknownAction = errors.New("unknown action")
	// ErrDuplicateKey means two spellings of one key, such as C-b and Ctrl+b.
	ErrDuplicateKey = errors.New("duplicate key")
	// ErrPrefixShadows means a prefix that is also a default binding.
	ErrPrefixShadows = errors.New("prefix hides a default key")
)

// Error reports a config value that the table cannot use. The message names
// the setting, so the user can find it in the file.
type Error struct {
	Setting string // "keys.prefix" or "keys.bind"
	Key     string // the key as the user wrote it
	Action  string // the action as the user wrote it; "" for the prefix
	Err     error  // ErrBadKey or ErrUnknownAction, with detail
}

// Error returns, for example `keys.bind "%": unknown action "split-sideways"`.
func (e *Error) Error() string {
	return fmt.Sprintf("%s %q: %v", e.Setting, e.Key, e.Err)
}

// Unwrap returns the failure kind.
func (e *Error) Unwrap() error { return e.Err }
