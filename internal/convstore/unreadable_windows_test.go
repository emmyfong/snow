package convstore

import (
	"testing"

	"golang.org/x/sys/windows"
)

// makeUnreadable makes the file at path one the user cannot open, until
// the test ends: it holds the file open with no sharing, as an editor or a
// scanner can. It returns the error that a read of it wraps.
func makeUnreadable(t *testing.T, path string) error {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(h) })
	return windows.ERROR_SHARING_VIOLATION
}
