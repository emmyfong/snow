package convstore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// maxID is the longest conversation ID that CheckID accepts.
const maxID = 64

// ErrBadID is the cause of an error for a conversation ID that is not a safe
// file name.
var ErrBadID = errors.New("not a valid conversation ID")

// NewID returns a new conversation ID: the UTC time and 8 random hex
// characters, for example "20261023-191500-a1b2c3d4". IDs sort by time and
// are safe file names on every OS, including file systems that ignore case.
func NewID(now time.Time) string {
	var b [4]byte
	// crypto/rand.Read never fails on the OSes Snow runs on.
	_, _ = rand.Read(b[:])
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// CheckID returns an error wrapping ErrBadID unless id is 1 to 64 lowercase
// letters, digits, and hyphens, starts with a letter or digit, and is not a
// Windows device name such as "con" or "com1".
func CheckID(id string) error {
	if id == "" || len(id) > maxID || id[0] == '-' {
		return fmt.Errorf("%w: %q", ErrBadID, id)
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return fmt.Errorf("%w: %q", ErrBadID, id)
		}
	}
	if isDevice(id) {
		return fmt.Errorf("%w: %q is a Windows device name", ErrBadID, id)
	}
	return nil
}

// isDevice reports whether id is a name that Windows opens as a device in
// any folder, whatever the extension.
func isDevice(id string) bool {
	switch id {
	case "con", "prn", "aux", "nul", "conin$", "conout$":
		return true
	}
	if len(id) == 4 && (strings.HasPrefix(id, "com") || strings.HasPrefix(id, "lpt")) {
		return id[3] >= '0' && id[3] <= '9'
	}
	return false
}
