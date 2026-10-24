package convstore

import (
	"errors"
	"fmt"
)

// ErrInUse is the cause of an error when a conversation already has a live
// writer in this process. Only one writer may append to a file.
var ErrInUse = errors.New("conversation is open in another writer")

// WriteError reports that a record could not be saved. The caller must stop
// before it sends anything that depends on the record.
type WriteError struct {
	Path string
	Err  error
}

func (e *WriteError) Error() string { return fmt.Sprintf("save conversation %s: %v", e.Path, e.Err) }

// Unwrap returns the underlying error.
func (e *WriteError) Unwrap() error { return e.Err }
