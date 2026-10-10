package config

import (
	"fmt"
	"strings"
)

// Error reports a config file that Snow cannot use. Callers can show it to
// the user as is: the message names the file, the line, and the key.
type Error struct {
	Path string // the config file
	Line int    // the line of the problem, from 1; 0 when unknown
	Key  string // the dotted TOML key, such as "scrollback.lines"; "" when unknown
	Msg  string // what is wrong
	Err  error  // the underlying cause, such as a read or TOML error; may be nil
}

// Error returns the message, for example
// `~/.snow/config.toml: line 3: key "keys.prefx": unknown key`.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Path)
	if e.Line > 0 {
		fmt.Fprintf(&b, ": line %d", e.Line)
	}
	if e.Key != "" {
		fmt.Fprintf(&b, ": key %q", e.Key)
	}
	b.WriteString(": ")
	b.WriteString(e.Msg)
	return b.String()
}

// Unwrap returns the underlying cause, so errors.Is and errors.As see it.
func (e *Error) Unwrap() error { return e.Err }
