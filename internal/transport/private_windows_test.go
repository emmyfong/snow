package transport

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSocketFolderPrivate(t *testing.T) {
	path := socketPath(t)
	listen(t, path)
	sd, err := windows.GetNamedSecurityInfo(filepath.Dir(path), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	// Read back as SDDL, for example D:PAI(A;OICI;FA;;;S-1-5-21-...).
	// P means protected from the parent's entries; Windows adds AI itself.
	got := sd.String()
	flags, aces, ok := strings.Cut(strings.TrimPrefix(got, "D:"), "(")
	if !ok || !strings.Contains(flags, "P") {
		t.Fatalf("folder ACL %s is not protected from inherited entries", got)
	}
	// Windows abbreviates well-known accounts in SDDL (the built-in
	// Administrator prints as LA), so format the expected entry the same way.
	want, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	_, wantACE, _ := strings.Cut(want.String(), "(")
	if aces != wantACE {
		t.Fatalf("folder ACL %s, want exactly one entry (%s): full control for %s", got, wantACE, sid)
	}
}
