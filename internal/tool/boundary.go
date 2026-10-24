package tool

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Pather is a tool whose input names paths. The approval step asks before
// such a tool touches a path outside the conversation folder.
type Pather interface {
	// Paths returns the paths the input names, as the model wrote them.
	Paths(input json.RawMessage) ([]string, error)
}

// Contains reports whether p is inside the folder, the folder itself
// included. On the OS Snow runs on, symlinks are resolved first, so a link
// inside that points outside is outside, and Windows paths compare without
// regard to case. A WSL folder compares its Linux paths only: the links
// live inside the distro.
func (f Folder) Contains(p Path) bool {
	if f.Distro != "" {
		return within(path.Clean(f.Path), p.Logical, "/", false)
	}
	root := realPath(filepath.Clean(f.Path))
	return within(root, realPath(p.OS), string(filepath.Separator), runtime.GOOS == "windows")
}

// within reports whether p is root or under it.
func within(root, p, sep string, fold bool) bool {
	if fold {
		root, p = strings.ToLower(root), strings.ToLower(p)
	}
	if p == root {
		return true
	}
	if !strings.HasSuffix(root, sep) {
		root += sep
	}
	return strings.HasPrefix(p, root)
}

// realPath resolves the symlinks of p. A path that does not exist yet,
// such as a file write will create, resolves through its nearest parent
// that does.
func realPath(p string) string {
	var rest []string
	for {
		r, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(append([]string{r}, rest...)...)
		}
		parent := filepath.Dir(p)
		if !errors.Is(err, fs.ErrNotExist) || parent == p {
			return filepath.Join(append([]string{p}, rest...)...)
		}
		rest = append([]string{filepath.Base(p)}, rest...)
		p = parent
	}
}

// Outside returns the paths of a call to t that fall outside the folder.
// A path the input names but the folder cannot resolve counts as outside,
// so the user is asked rather than the call let through.
func (f Folder) Outside(t Tool, input json.RawMessage) []string {
	pt, ok := t.(Pather)
	if !ok {
		return nil
	}
	paths, err := pt.Paths(input)
	if err != nil {
		return nil // the tool reports the bad input when it runs
	}
	var out []string
	for _, raw := range paths {
		p, err := f.Resolve(raw)
		if err != nil || !f.Contains(p) {
			out = append(out, raw)
		}
	}
	return out
}

// pathField returns the "path" field of an input, with def when it is
// missing or empty. Tools with one path use it as their Paths.
func pathField(input json.RawMessage, def string) ([]string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, err
	}
	if in.Path == "" {
		if def == "" {
			return nil, nil
		}
		in.Path = def
	}
	return []string{in.Path}, nil
}
