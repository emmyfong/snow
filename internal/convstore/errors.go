package convstore

import (
	"errors"
	"fmt"
)

// ErrBadID is the cause of an error for a conversation ID that is not a safe
// file name.
var ErrBadID = errors.New("not a valid conversation ID")

// ErrInUse is the cause of an error when a conversation already has a live
// writer in this Store. Only one writer may append to a file.
var ErrInUse = errors.New("conversation is open in another writer")

// WriteError reports that a record could not be saved. The caller must stop
// before it sends anything that depends on the record.
type WriteError struct {
	Path string
	Err  error
}

// Error implements error.
func (e *WriteError) Error() string { return fmt.Sprintf("save conversation %s: %v", e.Path, e.Err) }

// Unwrap returns the underlying error.
func (e *WriteError) Unwrap() error { return e.Err }

// Errors that Load and List wrap in a *LoadError.
var (
	ErrNewerVersion = errors.New("file is from a newer version of Snow")
	ErrNoMeta       = errors.New("file does not start with a meta record")
	ErrIDMismatch   = errors.New("meta record names another conversation")
)

// LoadError reports a conversation file that cannot be loaded. The file is
// kept as it is, for the user.
type LoadError struct {
	Path string
	Err  error
}

// Error implements error.
func (e *LoadError) Error() string { return fmt.Sprintf("load conversation %s: %v", e.Path, e.Err) }

// Unwrap returns the underlying error.
func (e *LoadError) Unwrap() error { return e.Err }
