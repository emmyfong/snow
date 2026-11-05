package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emmyfong/snow/internal/private"
)

// ProjectVersion is the project layout format version that SaveLayout
// writes. LoadLayout rejects a missing or newer one.
const ProjectVersion = 1

// maxProjectFile bounds a project layout file read back.
const maxProjectFile = 1 << 20

// DefaultProjectsDir is ~/.snow/projects on every OS.
func DefaultProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home folder: %w", err)
	}
	return filepath.Join(home, ".snow", "projects"), nil
}

// Projects keeps one window layout per project, one file per project key.
// The layout is encoded by internal/layout; the store does not read it.
// Callers serialize writes to the same key; ProjectSaver does.
type Projects struct {
	dir string
}

// projectFile is what a project layout file holds. Key is the project key
// in full, since the file name is only its hash.
type projectFile struct {
	Version int             `json:"version"`
	Key     string          `json:"key"`
	Layout  json.RawMessage `json:"layout"`
}

// OpenProjects returns Projects for dir and creates the folder, private to
// the current user, if it is missing.
func OpenProjects(dir string) (*Projects, error) {
	// A file names a project folder: the folder is the user's alone.
	if err := private.Dir(dir); err != nil {
		return nil, fmt.Errorf("create projects folder: %w", err)
	}
	return &Projects{dir: dir}, nil
}

// path is the file for key: a hash, so any folder makes a safe name.
func (p *Projects) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(p.dir, hex.EncodeToString(sum[:16])+".json")
}

// SaveLayout writes layout for key atomically, with mode 0600. It does not
// replace a file a newer Snow wrote: that gives ErrNewerVersion.
func (p *Projects) SaveLayout(key string, layout json.RawMessage) error {
	if _, err := p.LoadLayout(key); errors.Is(err, ErrNewerVersion) {
		return err
	}
	b, err := json.Marshal(projectFile{Version: ProjectVersion, Key: key, Layout: layout})
	if err != nil {
		return fmt.Errorf("encode project layout: %w", err)
	}
	if err := private.WriteAtomic(p.path(key), private.Bytes(b)); err != nil {
		return fmt.Errorf("save project layout: %w", err)
	}
	return nil
}

// LoadLayout reads the layout saved for key. A missing file gives an error
// that matches fs.ErrNotExist; a file that cannot be read back gives a
// *DecodeError with its path, and the file stays on disk.
func (p *Projects) LoadLayout(key string) (json.RawMessage, error) {
	path := p.path(key)
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case !fi.Mode().IsRegular():
		return nil, &DecodeError{Path: path, Err: errors.New("not a regular file")}
	case fi.Size() > maxProjectFile:
		return nil, &DecodeError{Path: path, Err: fmt.Errorf("%d bytes, more than %d", fi.Size(), maxProjectFile)}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f projectFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, &DecodeError{Path: path, Err: err}
	}
	switch {
	case f.Version == 0:
		return nil, &DecodeError{Path: path, Err: errors.New("no format version")}
	case f.Version > ProjectVersion:
		return nil, fmt.Errorf("%w: project layout format %d, this one reads up to %d", ErrNewerVersion, f.Version, ProjectVersion)
	case f.Key != key:
		return nil, &DecodeError{Path: path, Err: fmt.Errorf("holds project %q", f.Key)}
	case len(f.Layout) == 0:
		return nil, &DecodeError{Path: path, Err: errors.New("no layout")}
	}
	return f.Layout, nil
}
