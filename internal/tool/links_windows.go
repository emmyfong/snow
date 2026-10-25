package tool

import (
	"strings"

	"golang.org/x/sys/windows"
)

// volumeNameDOS asks GetFinalPathNameByHandle for a drive-letter path.
// golang.org/x/sys/windows does not name it.
const volumeNameDOS = 0x0

// evalLinks returns p with every link resolved. filepath.EvalSymlinks does
// not follow junctions, so Windows asks for the final path of the opened
// file, which follows symlinks and junctions alike.
func evalLinks(p string) (string, error) {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	// FILE_FLAG_BACKUP_SEMANTICS lets CreateFile open a folder.
	h, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), volumeNameDOS)
	if err != nil {
		return "", err
	}
	final := windows.UTF16ToString(buf[:n])
	if rest, ok := strings.CutPrefix(final, `\\?\UNC\`); ok {
		return `\\` + rest, nil
	}
	return strings.TrimPrefix(final, `\\?\`), nil
}
