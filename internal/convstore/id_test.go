package convstore

import (
	"errors"
	"regexp"
	"testing"
)

func TestNewID(t *testing.T) {
	id := NewID(at)
	if !regexp.MustCompile(`^20261023-191500-[0-9a-f]{8}$`).MatchString(id) {
		t.Fatalf("NewID = %q", id)
	}
	if err := CheckID(id); err != nil {
		t.Fatal(err)
	}
	if NewID(at) == id {
		t.Fatal("two IDs at the same second are equal")
	}
}

// TestBadID: an ID that is not a safe file name never reaches the file
// system.
func TestBadID(t *testing.T) {
	s := openStore(t)
	bad := []string{
		"", "..", ".", "a/b", `a\b`, "../x", "a.jsonl", "ABC", "-x", "a b", "a:b",
		"con", "nul", "com1", "lpt9", "conin$",
		"0123456789012345678901234567890123456789012345678901234567890123x",
	}
	for _, id := range bad {
		if _, err := s.Writer(id); !errors.Is(err, ErrBadID) {
			t.Errorf("Writer(%q) error %v, want ErrBadID", id, err)
		}
		if _, err := s.Path(id); !errors.Is(err, ErrBadID) {
			t.Errorf("Path(%q) error %v, want ErrBadID", id, err)
		}
	}
	for _, id := range []string{"a", "console", "com", "com10", testID} {
		if err := CheckID(id); err != nil {
			t.Errorf("CheckID(%q) = %v", id, err)
		}
	}
}
