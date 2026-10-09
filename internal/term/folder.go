package term

import (
	"net/url"
	"strings"
)

// oscFolder is the OSC command a shell uses to report its current folder.
const oscFolder = 7

// parseOSC7 extracts the path from an OSC 7 payload such as
// "file://host/home/emmy". It accepts the payload with or without the
// leading "7;", percent-encoded or raw paths, and Windows drive paths with
// either slash.
func parseOSC7(data []byte) (string, bool) {
	s := strings.TrimPrefix(string(data), "7;")
	rest, ok := strings.CutPrefix(s, "file://")
	if !ok {
		return "", false
	}
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return "", false
	}
	path := rest[i:]
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
	// A Windows drive path arrives as /C:/... (PowerShell) or /C:\... (cmd).
	// Return it in native form, so one folder looks the same from any shell.
	if len(path) >= 3 && path[0] == '/' && isDriveLetter(path[1]) && path[2] == ':' {
		path = strings.ReplaceAll(path[1:], "/", `\`)
	}
	return path, true
}

func isDriveLetter(c byte) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }

// Folder returns the program's current folder: the last one its shell
// reported through OSC 7, or what the OS reports for the process. It is empty
// when neither is known.
func (p *Pane) Folder() string {
	p.mu.Lock()
	folder := p.folder
	p.mu.Unlock()
	if folder != "" {
		return folder
	}
	if c, ok := p.pty.(interface{ cwd() string }); ok {
		return c.cwd()
	}
	return ""
}
