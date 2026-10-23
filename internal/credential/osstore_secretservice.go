//go:build !windows && !darwin

package credential

import (
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/zalando/go-keyring"
)

// unavailable reports whether err means there is no Secret Service to
// reach. A D-Bus error from the service itself, such as a locked or
// refused collection, is a real failure; so is a dismissed unlock prompt.
// Failing to reach the bus, or a bus with no Secret Service, means none.
func unavailable(err error) bool {
	if errors.Is(err, keyring.ErrUnsupportedPlatform) {
		return true
	}
	var de dbus.Error
	if errors.As(err, &de) {
		return de.Name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
			de.Name == "org.freedesktop.DBus.Error.NameHasNoOwner"
	}
	return !strings.HasPrefix(err.Error(), "failed to unlock")
}
