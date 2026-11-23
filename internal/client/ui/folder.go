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

// FolderTail returns the last two elements of a folder path, accepting both
// Windows and Unix separators. An empty path or filesystem root has no tail.
func FolderTail(folder string) string {
	parts := strings.FieldsFunc(strings.ReplaceAll(PlainLine(folder), `\`, "/"), func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts[max(len(parts)-2, 0):], "/")
}

// FolderShort returns the last two elements of folder, prefixed by "…/" when
// earlier elements were omitted. It returns "/" for a root and "-" for no folder.
func FolderShort(folder string) string {
	parts := strings.FieldsFunc(strings.ReplaceAll(PlainLine(folder), `\`, "/"), func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return FolderName(folder)
	}
	tail := strings.Join(parts[max(len(parts)-2, 0):], "/")
	if len(parts) > 2 {
		return "…/" + tail
	}
	return tail
}
