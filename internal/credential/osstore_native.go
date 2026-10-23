//go:build windows || darwin

package credential

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// unavailable reports whether err means the system has no store. Windows
// and macOS always have one, so other errors, such as a locked Keychain,
// are real failures.
func unavailable(err error) bool {
	return errors.Is(err, keyring.ErrUnsupportedPlatform)
}
