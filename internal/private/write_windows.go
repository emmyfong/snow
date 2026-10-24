package private

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// renameTries bounds the retries: about 0.5 s in total.
const renameTries = 10

// Rename replaces to with from. Go's os.Rename uses MoveFileEx with
// MOVEFILE_REPLACE_EXISTING, which replaces the file in one step. It fails
// while another process holds the target open without FILE_SHARE_DELETE:
// a reader, a virus scanner, or the search indexer. Those holds are short,
// so rename retries for a moment before it gives up.
func Rename(from, to string) error {
	var err error
	for i := range renameTries {
		err = os.Rename(from, to)
		if err == nil || !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
	}
	return err
}

// Remove deletes path, retrying while another process holds it open, as
// Rename does.
func Remove(path string) error {
	var err error
	for i := range renameTries {
		err = os.Remove(path)
		if err == nil || !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
	}
	return err
}

// SyncDir does nothing: Windows cannot flush a folder handle, and NTFS
// journals renames and new files itself.
func SyncDir(string) error { return nil }
