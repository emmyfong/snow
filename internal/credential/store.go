package credential

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Source is where a key was found.
type Source int

// The places a key can come from, in lookup order.
const (
	SourceEnv  Source = iota + 1 // the server's environment
	SourceOS                     // the OS credential store
	SourceFile                   // the private credentials file
)

// String names the place, for messages.
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
	ErrNoKey = errors.New("nothing saved")
	// ErrInEnvironment means the key comes from an environment variable,
	// which Snow cannot change; the user unsets it.
	ErrInEnvironment = errors.New("the key is set in the environment")
)

// fileName is the credentials file inside the store's folder.
const fileName = "credentials"

// osTimeout bounds one OS store call: a locked keyring can wait on an
// unlock prompt that never comes, for example in a session with no
// desktop.
const osTimeout = 5 * time.Second

// Store finds and saves API keys per provider name. It is safe for use from
// several goroutines. An OS store call can wait up to five seconds, so the
// server calls a Store off its state goroutine (ADR 0007).
type Store struct {
	lookupEnv func(string) (string, bool)
	osStore   OSStore // nil when the system has none
	dir       string
	notice    func(string)
	timeout   time.Duration

	// mu guards the probe result and the file. OS store calls run
	// without it, so a slow keyring cannot block a file lookup.
	mu         sync.Mutex
	probed     bool
	osWorks    bool
	osCause    error // why the OS store is unavailable
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
	return &Store{lookupEnv: lookupEnv, osStore: osStore, dir: dir, notice: notice, timeout: osTimeout}
}

// EnvVar is the environment variable that holds provider's key, for
// example ANTHROPIC_API_KEY.
func EnvVar(provider string) string {
	return strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_API_KEY"
}

// env returns provider's key from the environment, without the spaces or
// line ending a pasted value can carry.
func (s *Store) env(provider string) (string, bool) {
	v, ok := s.lookupEnv(EnvVar(provider))
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

// Get returns provider's key and where it was found: the environment, then
// the OS store, then the file. It returns ErrNoKey when none has it.
func (s *Store) Get(provider string) (Key, Source, error) {
	if v, ok := s.env(provider); ok {
		k, err := NewKey(v)
		if err != nil {
			return Key{}, 0, fmt.Errorf("%s: %w", EnvVar(provider), err)
		}
		return k, SourceEnv, nil
	}
	works, err := s.osAvailable()
	if err != nil {
		return Key{}, 0, err
	}
	if works {
		v, err := s.osGet(provider)
		switch {
		case err == nil:
			k, err := NewKey(strings.TrimSpace(v))
			if err != nil {
				return Key{}, 0, fmt.Errorf("OS credential store: %w", err)
			}
			return k, SourceOS, nil
		case !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnavailable):
			return Key{}, 0, fmt.Errorf("read the OS credential store: %w", err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return Key{}, 0, fmt.Errorf("%w for %s; set %s", ErrNoKey, provider, EnvVar(provider))
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
// has no store, and returns where it went. A key saved in the OS store
// leaves no copy in the file.
func (s *Store) Set(provider string, k Key) (Source, error) {
	if k.value == "" {
		return 0, ErrBadKey
	}
	works, err := s.osAvailable()
	if err != nil {
		return 0, err
	}
	if works {
		err := s.osSet(provider, k.value)
		if err == nil {
			if err := s.removeFromFile(provider); err != nil {
				return SourceOS, fmt.Errorf("saved in the OS credential store, but the old copy stays: %w", err)
			}
			return SourceOS, nil
		}
		if !errors.Is(err, ErrUnavailable) {
			return 0, fmt.Errorf("save to the OS credential store: %w", err)
		}
		s.mu.Lock()
		s.osCause = err
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keys, err := readFile(s.path())
	if err != nil {
		return 0, err
	}
	keys[provider] = k.value
	if err := writeFile(s.dir, s.path(), keys); err != nil {
		return 0, err
	}
	s.noticeOnce.Do(func() {
		s.notice(fmt.Sprintf("API keys are saved in plain text in %s, readable only by you. Reason: %v.", s.path(), s.osCause))
	})
	return SourceFile, nil
}

// Delete removes provider's saved key from the OS store and the file. A
// key in the environment cannot be removed: Delete returns
// ErrInEnvironment. With no saved key it returns ErrNoKey.
func (s *Store) Delete(provider string) error {
	if _, ok := s.env(provider); ok {
		return fmt.Errorf("%w (%s)", ErrInEnvironment, EnvVar(provider))
	}
	works, err := s.osAvailable()
	if err != nil {
		return err
	}
	removed := false
	if works {
		switch err := s.osDelete(provider); {
		case err == nil:
			removed = true
		case !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnavailable):
			return fmt.Errorf("delete from the OS credential store: %w", err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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

// removeFromFile drops provider's line from the file, if it has one.
func (s *Store) removeFromFile(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys, err := readFile(s.path())
	if err != nil {
		return err
	}
	if _, ok := keys[provider]; !ok {
		return nil
	}
	delete(keys, provider)
	return writeFile(s.dir, s.path(), keys)
}

func (s *Store) path() string { return filepath.Join(s.dir, fileName) }
