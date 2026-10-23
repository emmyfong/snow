package credential

import (
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestFileMode: the credentials file's access list has one entry, for the
// current user, so no other account can read it.
func TestFileMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".snow")
	writeKeys(t, dir, map[string]string{"anthropic": secret})
	sd, err := windows.GetNamedSecurityInfo(filepath.Join(dir, fileName), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
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
		t.Fatalf("access list has %d entries, want 1: %s", dacl.AceCount, sd)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.Equals(user.User.Sid) {
		t.Fatalf("the entry is for %s, want the current user %s", sid, user.User.Sid)
	}
}
