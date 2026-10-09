package transport

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// DefaultPath is %LOCALAPPDATA%\snow\run\default.sock.
func DefaultPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find local app data folder: %w", err)
	}
	return filepath.Join(cache, "snow", "run", "default.sock"), nil
}

// makePrivateDir creates dir and replaces its access control list with one
// entry: full control for the current user, inherited by the socket inside.
// "P" protects the list from the parent folder's entries.
func makePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create socket folder: %w", err)
	}
	// SetNamedSecurityInfo follows symbolic links and junctions, so a reparse
	// point here would move the ACL change to another folder.
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("check socket folder: %w", err)
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("socket folder %s is not a plain folder", dir)
	}
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		return fmt.Errorf("build socket folder ACL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("build socket folder ACL: %w", err)
	}
	err = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
	if err != nil {
		return fmt.Errorf("restrict socket folder: %w", err)
	}
	return nil
}

func currentUserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("find current user: %w", err)
	}
	return user.User.Sid.String(), nil
}

// restrictSocket does nothing on Windows: the socket inherits the folder ACL.
func restrictSocket(string) error { return nil }
