package credential

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// OSStore is the system credential store: Windows Credential Manager, the
// macOS Keychain, or Secret Service on Linux. Get and Delete return an
// error matching ErrNotFound when no key is saved. Any call returns an
// error matching ErrUnavailable when the system has no store to reach.
type OSStore interface {
	Get(provider string) (string, error)
	Set(provider, key string) error
	Delete(provider string) error
}

var (
	// ErrNotFound means the OS store holds no key for the provider.
	ErrNotFound = errors.New("not in the OS credential store")
	// ErrUnavailable means the system has no OS store to reach, for
	// example Linux with no D-Bus session or no Secret Service, or the
	// store did not answer in time. Keys then go to the file.
	ErrUnavailable = errors.New("no OS credential store")
)

// service groups Snow's entries in the OS store.
const service = "snow"

// keyringStore is the OS store through go-keyring, which needs no cgo.
type keyringStore struct{}

func (keyringStore) Get(provider string) (string, error) {
	v, err := keyring.Get(service, provider)
	return v, osErr(err)
}

func (keyringStore) Set(provider, key string) error {
	return osErr(keyring.Set(service, provider, key))
}

func (keyringStore) Delete(provider string) error {
	return osErr(keyring.Delete(service, provider))
}

func osErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case unavailable(err):
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	default:
		return err
	}
}
