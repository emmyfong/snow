// Package private keeps the current user's files private: folders only the
// user can read (the socket folder, the saved sessions, the credentials
// file), and files replaced in one atomic step.
package private

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Dir creates dir and replaces its access control list with one
// entry: full control for the current user, inherited by everything inside.
// "P" protects the list from the parent folder's entries.
func Dir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	// SetNamedSecurityInfo follows symbolic links and junctions, so a reparse
	// point here would move the ACL change to another folder.
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("check folder: %w", err)
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("folder %s is not a plain folder", dir)
	}
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		return fmt.Errorf("build folder ACL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("build folder ACL: %w", err)
	}
	err = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
	if err != nil {
		return fmt.Errorf("restrict folder: %w", err)
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
