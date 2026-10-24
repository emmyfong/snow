// Package privatetest checks in tests that a file or folder is private to
// the current user.
package privatetest

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// OwnerOnly fails t unless the access list of path has one entry, for the
// current user, so no other account can read it. Windows has no mode bits,
// so want is not used; on Linux and macOS it is the mode to check.
func OwnerOnly(t testing.TB, path string, _ os.FileMode) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("access list of %s has %d entries, want 1: %s", path, dacl.AceCount, sd)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.Equals(user.User.Sid) {
		t.Fatalf("the entry of %s is for %s, want the current user %s", path, sid, user.User.Sid)
	}
}
