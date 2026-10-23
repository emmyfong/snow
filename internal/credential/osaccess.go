package credential

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// osAvailable reports whether to use the OS store. A lookup of a name no
// one saves tells a working store (not found) from a missing one, such as
// Secret Service in WSL with no D-Bus session. Only those two answers are
// kept. A store that is there but fails, for example a locked keyring,
// returns its error, so a key never goes to the file because of it; the
// next call asks again. A store that did not answer in time sends this
// call to the file and is asked again next time.
func (s *Store) osAvailable() (bool, error) {
	s.mu.Lock()
	if s.probed {
		defer s.mu.Unlock()
		return s.osWorks, nil
	}
	s.mu.Unlock()
	if s.osStore == nil {
		s.keep(false, ErrUnavailable)
		return false, nil
	}
	_, err := s.osGet(probeName)
	switch {
	case err == nil || errors.Is(err, ErrNotFound):
		s.keep(true, nil)
		return true, nil
	case errors.Is(err, errOSTimeout):
		s.mu.Lock()
		s.osCause = err
		s.mu.Unlock()
		return false, nil
	case errors.Is(err, ErrUnavailable):
		s.keep(false, err)
		return false, nil
	default:
		return false, fmt.Errorf("the OS credential store failed: %w", err)
	}
}

func (s *Store) keep(works bool, cause error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probed, s.osWorks, s.osCause = true, works, cause
}

// probeName is never saved, so looking it up only tests the store.
const probeName = "snow-probe"

// errOSTimeout means an OS store call did not return in time.
var errOSTimeout = fmt.Errorf("%w: it did not answer", ErrUnavailable)

func (s *Store) osGet(provider string) (string, error) {
	return osCall(s.timeout, func() (string, error) { return s.osStore.Get(provider) })
}

func (s *Store) osSet(provider, key string) error {
	_, err := osCall(s.timeout, func() (struct{}, error) { return struct{}{}, s.osStore.Set(provider, key) })
	return scrub(err, key)
}

// scrub hides key in err's text, for a store whose error repeats what it
// was given. errors.Is still sees the original, but nothing unwraps to it.
func scrub(err error, key string) error {
	if err == nil || !strings.Contains(err.Error(), key) {
		return err
	}
	return &scrubbed{msg: strings.ReplaceAll(err.Error(), key, hidden), orig: err}
}

type scrubbed struct {
	msg  string
	orig error
}

func (e *scrubbed) Error() string { return e.msg }

// Is matches what the original error matches, such as ErrUnavailable.
func (e *scrubbed) Is(target error) bool { return errors.Is(e.orig, target) }

func (s *Store) osDelete(provider string) error {
	_, err := osCall(s.timeout, func() (struct{}, error) { return struct{}{}, s.osStore.Delete(provider) })
	return err
}

// osCall runs f with a timeout. On a timeout f keeps running in the
// background until the OS returns; its result is dropped.
func osCall[T any](timeout time.Duration, f func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := f()
		done <- result{v, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.v, r.err
	case <-timer.C:
		var zero T
		return zero, errOSTimeout
	}
}
