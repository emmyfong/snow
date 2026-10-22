package private

import (
	"os"
	"testing"
	"time"
)

// Go opens files on Windows without FILE_SHARE_DELETE, so a reader that
// holds the snapshot open makes a rename over it fail for a moment. Virus
// scanners and indexers do the same.
func TestRenameWaitsForOpenTarget(t *testing.T) {
	path := oldFile(t)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(30*time.Millisecond, func() { _ = f.Close() })
	if err := WriteAtomic(path, Bytes(newData)); err != nil {
		t.Fatalf("write over a briefly open file: %v", err)
	}
	assertContent(t, path, newData)
}
