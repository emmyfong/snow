// Package credential keeps API keys: in the server's environment, in the
// OS credential store, or, where no store exists, in a private file. A key
// leaves the package only as a Key, which prints as "[hidden]", so it
// cannot reach a log line or an error text by accident.
package credential

import (
	"errors"
	"fmt"
	"strings"
)

// Key is an API key. Every fmt verb prints it as "[hidden]"; only Reveal
// returns the value.
type Key struct{ value string }

// hidden is what a Key prints as.
const hidden = "[hidden]"

// ErrBadKey means a key is empty or holds white space, so it cannot be
// stored. The error never holds the key.
var ErrBadKey = errors.New("an API key must be one word with no spaces")

// NewKey checks s and wraps it.
func NewKey(s string) (Key, error) {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return Key{}, ErrBadKey
	}
	return Key{value: s}, nil
}

// Reveal returns the key's value, to send to its provider.
func (k Key) Reveal() string { return k.value }

func (k Key) String() string { return hidden }

// GoString hides the key from %#v.
func (k Key) GoString() string { return hidden }

// Format hides the key from every verb, including %q and %x.
func (k Key) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(hidden)) }
