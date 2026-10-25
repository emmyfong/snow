package tool

import (
	"errors"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Folder is a conversation folder: a path, and the WSL distribution it
// belongs to, if any. A WSL folder and its Windows view are different
// folders. Path is in the folder's own form: a Linux path for WSL.
type Folder struct {
	Path   string
	Distro string // empty for a folder of the OS Snow runs on
}

// Path is a path a tool works on, in two forms.
type Path struct {
	// Logical is the cleaned path in the folder's own form, a Linux path
	// for a WSL folder. The folder boundary is checked on it.
	Logical string
	// OS is where Snow opens the path: the same as Logical, or for a WSL
	// folder on Windows, its \\wsl.localhost path.
	OS string
}

// errEmptyPath is the cause of an error for a tool call with no path.
var errEmptyPath = errors.New("no path given")

// Resolve turns a path from the model, relative to the folder or absolute,
// into a Path.
func (f Folder) Resolve(p string) (Path, error) {
	if p == "" {
		return Path{}, errEmptyPath
	}
	if f.Distro == "" {
		if !filepath.IsAbs(p) {
			p = filepath.Join(f.Path, p)
		}
		p = filepath.Clean(p)
		return Path{Logical: p, OS: p}, nil
	}
	p = strings.ReplaceAll(p, `\`, "/")
	if !path.IsAbs(p) {
		p = path.Join(f.Path, p)
	}
	p = path.Clean(p)
	os := p
	if runtime.GOOS == "windows" {
		os = WSLToUNC(f.Distro, p)
	}
	return Path{Logical: p, OS: os}, nil
}

// uncPrefix starts every path the Windows side uses to reach a WSL
// distribution's files.
const uncPrefix = `\\wsl.localhost\`

// WSLToUNC returns the Windows path of a Linux path in a WSL distribution,
// for example \\wsl.localhost\Ubuntu\home\u for /home/u.
func WSLToUNC(distro, linux string) string {
	return uncPrefix + distro + strings.ReplaceAll(path.Clean("/"+linux), "/", `\`)
}

// UNCToWSL is the reverse of WSLToUNC. It also takes the older \\wsl$
// form. ok is false for a path that is not in a WSL distribution.
func UNCToWSL(unc string) (distro, linux string, ok bool) {
	rest, found := cutPrefixFold(unc, uncPrefix)
	if !found {
		if rest, found = cutPrefixFold(unc, `\\wsl$\`); !found {
			return "", "", false
		}
	}
	distro, tail, _ := strings.Cut(rest, `\`)
	if distro == "" {
		return "", "", false
	}
	return distro, path.Clean("/" + strings.ReplaceAll(tail, `\`, "/")), true
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}
