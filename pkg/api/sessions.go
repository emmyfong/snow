package api

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxSessionName bounds a session name, in bytes.
const MaxSessionName = 64

// ErrInvalidSessionRequest is returned by the Validate methods of the
// session requests (KillSession and RenameSession).
var ErrInvalidSessionRequest = errors.New("api: invalid session request")

// Detach leaves the attached session without closing the connection; the
// session keeps running. The server answers with a SessionList, as it does
// for any client that has not attached, so the client can show the home
// page.
type Detach struct{}

// KillSession ends a session and every program in it. Clients attached to
// it receive an Error with CodeSessionEnded. The server answers the sender
// with an Error when there is no such session.
type KillSession struct {
	Session string `json:"session"`
}

// RenameSession renames session Session to Name. Clients attached to it
// receive a Layout with the new name. The server answers with an Error when
// the session does not exist or the name is taken.
type RenameSession struct {
	Session string `json:"session"`
	Name    string `json:"name"`
}

// Validate checks that the request names a session.
func (k *KillSession) Validate() error {
	if k.Session == "" {
		return fmt.Errorf("%w: no session given", ErrInvalidSessionRequest)
	}
	return nil
}

// Validate checks that the request names a session and that the new name
// is usable: not empty, no longer than MaxSessionName, with no space at
// either end and no control characters.
func (r *RenameSession) Validate() error {
	if r.Session == "" {
		return fmt.Errorf("%w: no session given", ErrInvalidSessionRequest)
	}
	return ValidSessionName(r.Name)
}

// ValidSessionName checks a new session name; see RenameSession.Validate.
func ValidSessionName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%w: the name is empty", ErrInvalidSessionRequest)
	case len(name) > MaxSessionName:
		return fmt.Errorf("%w: the name is longer than %d bytes", ErrInvalidSessionRequest, MaxSessionName)
	case !utf8.ValidString(name):
		return fmt.Errorf("%w: the name is not valid UTF-8", ErrInvalidSessionRequest)
	case strings.TrimSpace(name) != name:
		return fmt.Errorf("%w: the name starts or ends with a space", ErrInvalidSessionRequest)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return fmt.Errorf("%w: the name has a control character", ErrInvalidSessionRequest)
	}
	return nil
}

// Type implements Message.
func (*Detach) Type() MsgType { return TypeDetach }
func (*Detach) message()      {}

// Type implements Message.
func (*KillSession) Type() MsgType { return TypeKillSession }
func (*KillSession) message()      {}

// Type implements Message.
func (*RenameSession) Type() MsgType { return TypeRenameSession }
func (*RenameSession) message()      {}
