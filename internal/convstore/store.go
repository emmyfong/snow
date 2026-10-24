package convstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/emmyfong/snow/internal/private"
)

// ext is the extension of a conversation file.
const ext = ".jsonl"

// DefaultDir returns ~/.snow/conversations.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home folder: %w", err)
	}
	return filepath.Join(home, ".snow", "conversations"), nil
}

// Store is a folder of conversation files.
type Store struct {
	dir string

	mu   sync.Mutex
	live map[string]bool // IDs with an open Writer
}

// Open returns the store in dir, and creates dir, private to the user.
func Open(dir string) (*Store, error) {
	if err := private.Dir(dir); err != nil {
		return nil, fmt.Errorf("open conversations: %w", err)
	}
	return &Store{dir: dir, live: map[string]bool{}}, nil
}

// Path returns the file of the conversation id. A bad id gives an error
// wrapping ErrBadID, so it never reaches the file system.
func (s *Store) Path(id string) (string, error) {
	if err := CheckID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, id+ext), nil
}

// claim marks id as having a live writer, or fails if it has one.
func (s *Store) claim(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.live[id] {
		return fmt.Errorf("%w: %s", ErrInUse, id)
	}
	s.live[id] = true
	return nil
}

func (s *Store) release(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.live, id)
}
