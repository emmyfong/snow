package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// Locate returns an *Error for a setting that a later stage rejected, for
// example a key binding that names no action. It finds the setting's line in
// the file at path; key is the setting's path in the file, such as
// {"keys", "bind", "%"}. Locate never fails: if the file cannot be read, the
// error has no line.
func Locate(path string, key []string, msg string, cause error) *Error {
	e := &Error{Path: path, Msg: msg, Err: cause}
	parts := make([]string, len(key))
	for i, k := range key {
		parts[i] = quoteKey(k)
	}
	e.Key = strings.Join(parts, ".")
	if data, err := os.ReadFile(path); err == nil {
		e.Line = keyLine(string(data), key)
	}
	return e
}

// typeErrorText matches the text of a TOML type mismatch, for example
// `toml: line 3 (last key "agent.x.env.A"): incompatible types: ...`.
var typeErrorText = regexp.MustCompile(`^toml: line (\d+) \(last key "([^"]*)"\): (.*)$`)

// typeError turns a TOML decode error that is not a ParseError into an
// *Error with the line and the key its text names.
func typeError(err error) *Error {
	m := typeErrorText.FindStringSubmatch(err.Error())
	if m == nil {
		return &Error{Msg: err.Error(), Err: err}
	}
	line, _ := strconv.Atoi(m[1]) // the pattern holds only digits
	return &Error{Line: line, Key: m[2], Msg: m[3], Err: err}
}
