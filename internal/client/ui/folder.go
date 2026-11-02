package ui

import (
	"path"
	"strings"
)

// FolderName is the last element of a folder path, as Linux, macOS, WSL, or
// Windows writes it. The server may run on another OS than the client, such
// as in WSL, so both separators count. It returns "/" for a root path and "-"
// for no folder.
func FolderName(folder string) string {
	s := strings.TrimRight(strings.ReplaceAll(folder, `\`, "/"), "/")
	switch {
	case s != "":
		return path.Base(s)
	case folder != "":
		return "/"
	default:
		return "-"
	}
}
