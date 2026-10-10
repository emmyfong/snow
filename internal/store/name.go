package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrBadName reports a session name that cannot be a file name: empty, or
// too long once escaped.
var ErrBadName = errors.New("bad session name")

// maxFileName keeps file names under the 255-byte limit of common file
// systems.
const maxFileName = 255

// reserved are Windows device names. Windows refuses them as file names even
// with an extension, so "con.json" cannot exist.
var reserved = []string{
	"con", "prn", "aux", "nul",
	"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
	"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
}

// fileName maps a session name to its file name. Lowercase letters, digits,
// '-', and '_' stay as they are; every other byte becomes %XX. Escaping
// uppercase keeps "API" and "api" apart on file systems that ignore case,
// and the same name gives the same file on every OS.
func fileName(session string) (string, error) {
	if session == "" {
		return "", fmt.Errorf("%w: empty", ErrBadName)
	}
	var b strings.Builder
	for i := 0; i < len(session); i++ {
		c := session[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	base := b.String()
	if slices.Contains(reserved, base) {
		base = fmt.Sprintf("%%%02X", base[0]) + base[1:]
	}
	name := base + ".json"
	if len(name) > maxFileName {
		return "", fmt.Errorf("%w: %d bytes once escaped", ErrBadName, len(name))
	}
	return name, nil
}
