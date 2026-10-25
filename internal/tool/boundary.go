package tool

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
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

// realPath resolves the links of p: symlinks, and junctions on Windows. A
// path that does not exist yet, such as a file write will create, resolves
// through its nearest parent that does. A link whose target does not exist
// is followed by hand, so it cannot hide where it leads.
func realPath(p string) string { return resolveLinks(p, 0) }

// maxLinks bounds how many dangling links resolveLinks follows, so a loop
// of links ends.
const maxLinks = 40

func resolveLinks(p string, depth int) string {
	var rest []string
	join := func(base string) string { return filepath.Join(append([]string{base}, rest...)...) }
	for {
		r, err := evalLinks(p)
		if err == nil {
			return join(r)
		}
		if target, ok := danglingTarget(p); ok && depth < maxLinks {
			return resolveLinks(join(target), depth+1)
		}
		parent := filepath.Dir(p)
		if !errors.Is(err, fs.ErrNotExist) || parent == p {
			return join(p)
		}
		rest = append([]string{filepath.Base(p)}, rest...)
		p = parent
	}
}

// danglingTarget returns where the link p points, as an absolute path,
// when p is a link. Go reports a Windows junction as irregular, not as a
// symlink, but Readlink reads both.
func danglingTarget(p string) (string, bool) {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return "", false
	}
	target, err := os.Readlink(p)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(p), target)
	}
	return target, true
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
		// Input that names no readable path could name any path.
		return []string{unreadableInput}
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

// unreadableInput stands for the paths of input a tool cannot read.
const unreadableInput = "(input that could not be read)"

// pathField returns the "path" field of an input, with def when it is
// missing or empty. Tools with one path use it as their Paths.
func pathField(input json.RawMessage, def string) ([]string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := decodeOne(input, &in, false); err != nil {
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
