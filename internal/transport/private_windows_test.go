package transport

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// acl reads the DACL of path as SDDL, for example
// D:PAI(A;OICI;FA;;;S-1-5-21-...). Windows adds flags such as AI itself.
func acl(t *testing.T, path string) (flags, aces string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	flags, aces, ok := strings.Cut(strings.TrimPrefix(sd.String(), "D:"), "(")
	if !ok {
		t.Fatalf("%s has no ACL entries: %s", path, sd)
	}
	return flags, "(" + aces
}

// sddlEntry formats one ACL entry for the current user the way Windows prints
// it. Windows abbreviates well-known accounts: the built-in Administrator
// prints as LA, not its SID.
func sddlEntry(t *testing.T, rights string) string {
	t.Helper()
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:(A;" + rights + ";;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	_, entry, _ := strings.Cut(sd.String(), "(")
	return "(" + entry
}

func TestSocketFolderPrivate(t *testing.T) {
	path := socketPath(t)
	listen(t, path)

	flags, aces := acl(t, filepath.Dir(path))
	if !strings.Contains(flags, "P") {
		t.Fatalf("folder ACL is not protected from inherited entries: %s%s", flags, aces)
	}
	if want := sddlEntry(t, "OICI;FA"); aces != want {
		t.Fatalf("folder ACL %s, want exactly one entry %s", aces, want)
	}

	// The socket file inherits the folder's single entry (ID = inherited).
	if _, aces := acl(t, path); aces != sddlEntry(t, "ID;FA") {
		t.Fatalf("socket file ACL %s, want exactly one inherited entry %s", aces, sddlEntry(t, "ID;FA"))
	}
}

func TestDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LocalAppData", dir)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "snow", "run", "default.sock"); got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

// TestListenRejectsJunctionFolder uses a directory junction, which needs no
// special privilege, unlike a symbolic link.
func TestListenRejectsJunctionFolder(t *testing.T) {
	path := socketPath(t)
	target := t.TempDir()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Dir(path), target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J: %v: %s", err, out)
	}
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen accepted a symlinked socket folder")
	}
}
