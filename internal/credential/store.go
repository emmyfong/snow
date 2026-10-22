package credential

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Source is where a key was found.
type Source int

// The places a key can come from, in lookup order.
const (
	SourceEnv  Source = iota + 1 // the server's environment
	SourceOS                     // the OS credential store
	SourceFile                   // the private credentials file
)

func (s Source) String() string {
	switch s {
	case SourceEnv:
		return "environment"
	case SourceOS:
		return "OS credential store"
	case SourceFile:
		return "credentials file"
	default:
		return "unknown"
	}
}

var (
	// ErrNoKey means no key is saved for the provider.
	ErrNoKey = errors.New("no API key saved")
	// ErrInEnvironment means the key comes from an environment variable,
	// which Snow cannot change; the user unsets it.
	ErrInEnvironment = errors.New("the key is set in the environment")
)

// fileName is the credentials file inside the store's folder.
const fileName = "credentials"

// Store finds and saves API keys per provider name. It is safe for use from
// several goroutines.
type Store struct {
	lookupEnv func(string) (string, bool)
	osStore   OSStore // nil when the system has none
	dir       string
	notice    func(string)

	mu         sync.Mutex
	probed     bool
	osWorks    bool
	noticeOnce sync.Once
}

// New returns a Store that uses the OS credential store when it works and
// otherwise the file "credentials" in dir (for example ~/.snow). notice
// receives a one-time message when keys go to the file; it may be nil.
func New(dir string, notice func(string)) *Store {
	return newStore(os.LookupEnv, keyringStore{}, dir, notice)
}

func newStore(lookupEnv func(string) (string, bool), osStore OSStore, dir string, notice func(string)) *Store {
	if notice == nil {
		notice = func(string) {}
	}
	return &Store{lookupEnv: lookupEnv, osStore: osStore, dir: dir, notice: notice}
}

// EnvVar is the environment variable that holds provider's key, for
// example ANTHROPIC_API_KEY.
func EnvVar(provider string) string {
	return strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_API_KEY"
}

// Get returns provider's key and where it was found: the environment, then
// the OS store, then the file. It returns ErrNoKey when none has it.
func (s *Store) Get(provider string) (Key, Source, error) {
	if v, ok := s.lookupEnv(EnvVar(provider)); ok && v != "" {
		k, err := NewKey(v)
		if err != nil {
			return Key{}, 0, fmt.Errorf("%s: %w", EnvVar(provider), err)
		}
		return k, SourceEnv, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.osAvailable() {
		v, err := s.osStore.Get(provider)
		switch {
		case err == nil:
			k, err := NewKey(v)
			if err != nil {
				return Key{}, 0, fmt.Errorf("OS credential store: %w", err)
			}
			return k, SourceOS, nil
		case !errors.Is(err, ErrNotFound):
			return Key{}, 0, fmt.Errorf("read the OS credential store: %w", err)
		}
	}
	keys, err := readFile(s.path())
	if err != nil {
		return Key{}, 0, err
	}
	if v, ok := keys[provider]; ok {
		k, err := NewKey(v)
		if err != nil {
			return Key{}, 0, fmt.Errorf("%s: %w", s.path(), err)
		}
		return k, SourceFile, nil
	}
	return Key{}, 0, fmt.Errorf("%w for %s", ErrNoKey, provider)
}

// APIKey returns provider's key value, for providers that read keys
// through a narrow interface.
func (s *Store) APIKey(provider string) (string, error) {
	k, _, err := s.Get(provider)
	if err != nil {
		return "", err
	}
	return k.Reveal(), nil
}

// Set saves provider's key in the OS store, or in the file when the system
// has no store, and returns where it went.
func (s *Store) Set(provider string, k Key) (Source, error) {
	if k.value == "" {
		return 0, ErrBadKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.osAvailable() {
		if err := s.osStore.Set(provider, k.value); err != nil {
			return 0, fmt.Errorf("save to the OS credential store: %w", err)
		}
		return SourceOS, nil
	}
	s.noticeOnce.Do(func() {
		s.notice("No OS credential store found; API keys are saved in " + s.path() + ", readable only by you.")
	})
	keys, err := readFile(s.path())
	if err != nil {
		return 0, err
	}
	keys[provider] = k.value
	if err := writeFile(s.dir, s.path(), keys); err != nil {
		return 0, err
	}
	return SourceFile, nil
}

// Delete removes provider's saved key from the OS store and the file. A
// key in the environment cannot be removed: Delete returns
// ErrInEnvironment. With no saved key it returns ErrNoKey.
func (s *Store) Delete(provider string) error {
	if v, ok := s.lookupEnv(EnvVar(provider)); ok && v != "" {
		return fmt.Errorf("%w (%s)", ErrInEnvironment, EnvVar(provider))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := false
	if s.osAvailable() {
		switch err := s.osStore.Delete(provider); {
		case err == nil:
			removed = true
		case !errors.Is(err, ErrNotFound):
			return fmt.Errorf("delete from the OS credential store: %w", err)
		}
	}
	keys, err := readFile(s.path())
	if err != nil {
		return err
	}
	if _, ok := keys[provider]; ok {
		delete(keys, provider)
		if err := writeFile(s.dir, s.path(), keys); err != nil {
			return err
		}
		removed = true
	}
	if !removed {
		return fmt.Errorf("%w for %s", ErrNoKey, provider)
	}
	return nil
}

func (s *Store) path() string { return filepath.Join(s.dir, fileName) }

// osAvailable asks the OS store once whether it works. A lookup of a name
// no one saves tells a working store (not found) from a missing one, such
// as Secret Service in WSL with no D-Bus session. The caller holds s.mu.
func (s *Store) osAvailable() bool {
	if !s.probed {
		s.probed = true
		if s.osStore != nil {
			_, err := s.osStore.Get(probeName)
			s.osWorks = err == nil || errors.Is(err, ErrNotFound)
		}
	}
	return s.osWorks
}

// probeName is never saved, so looking it up only tests the store.
const probeName = "snow-probe"
