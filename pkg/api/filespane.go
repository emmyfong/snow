package api

import (
	"fmt"
	"path"
	"strings"
)

// PaneFiles is the files pane: the folder's tree, with a mark for each
// file git sees as changed. Its state is FilesState.
const PaneFiles = "files"

// The actions of a files pane, in SidePaneAction.Action.
const (
	FilesActExpand   = "expand"   // open the folder ArgPath and read its children
	FilesActCollapse = "collapse" // hide the folder ArgPath and its children
	FilesActRefresh  = "refresh"  // read the expanded folders again
	FilesActSelect   = "select"   // reveal ArgPath, opening its folders
)

// ArgPath is a file or folder path from the pane's folder, with forward
// slashes. A command's OpOpenFile takes it too.
const ArgPath = "path"

// ArgLine is a one-based line number in a file. It is optional.
const ArgLine = "line"

// Bounds on a files pane state. A folder with more entries than
// MaxFilesEntries says how many it left out.
const (
	MaxFilesEntries = 2000
	MaxFilesFolders = 256
)

// FilesState is the state of a files pane.
type FilesState struct {
	Folder  string `json:"folder"`
	Loading bool   `json:"loading,omitempty"` // the tree is read; the rest is the last answer
	// Expanded holds the folders whose children are shown, root first.
	// The root folder is the empty path.
	Expanded []string `json:"expanded,omitempty"`
	// Folders holds the read children of each expanded folder. A folder
	// outside Expanded is not shown.
	Folders []FilesFolder `json:"folders,omitempty"`
	// Selected is the file the server was asked to show, or "".
	Selected string `json:"selected,omitempty"`
	Omitted  int    `json:"omitted,omitempty"` // entries left out past the bounds
	// Error is the last failure, such as a folder that could not be read.
	Error string `json:"error,omitempty"`
}

// FilesFolder is one expanded folder's children.
type FilesFolder struct {
	Path    string       `json:"path"` // from the pane's folder; "" is the root
	Entries []FilesEntry `json:"entries,omitempty"`
}

// FilesEntry is one file or folder in the tree.
type FilesEntry struct {
	Name string `json:"name"`
	Path string `json:"path"` // from the pane's folder
	Dir  bool   `json:"dir,omitempty"`
	// Mark is git's one-letter change for the file: M, A, D, R, C, T, U,
	// or ? for untracked. Empty means no change.
	Mark string `json:"mark,omitempty"`
	// Deleted means the file is gone from the folder but git still lists
	// it as changed.
	Deleted bool `json:"deleted,omitempty"`
}

// Kind implements SideState.
func (*FilesState) Kind() string { return PaneFiles }

// Validate checks that st can be drawn.
func (st *FilesState) Validate() error {
	if len(st.Folders) > MaxFilesFolders {
		return fmt.Errorf("%w: %d folders, at most %d", ErrInvalidSidePane, len(st.Folders), MaxFilesFolders)
	}
	seen := map[string]bool{}
	for _, f := range st.Folders {
		if f.Path != "" && !validRel(f.Path) {
			return fmt.Errorf("%w: folder path %q", ErrInvalidSidePane, f.Path)
		}
		if seen[f.Path] {
			return fmt.Errorf("%w: folder %q twice", ErrInvalidSidePane, f.Path)
		}
		seen[f.Path] = true
		if len(f.Entries) > MaxFilesEntries {
			return fmt.Errorf("%w: folder %q has %d entries, at most %d", ErrInvalidSidePane, f.Path, len(f.Entries), MaxFilesEntries)
		}
		for _, e := range f.Entries {
			if e.Name == "" || strings.ContainsAny(e.Name, `/\`) {
				return fmt.Errorf("%w: entry name %q", ErrInvalidSidePane, e.Name)
			}
			if !validRel(e.Path) {
				return fmt.Errorf("%w: entry path %q", ErrInvalidSidePane, e.Path)
			}
			if e.Mark != "" && (len(e.Mark) != 1 || !strings.ContainsRune("MADRCTU?", rune(e.Mark[0]))) {
				return fmt.Errorf("%w: entry mark %q", ErrInvalidSidePane, e.Mark)
			}
		}
	}
	for _, p := range st.Expanded {
		if p != "" && !validRel(p) {
			return fmt.Errorf("%w: expanded path %q", ErrInvalidSidePane, p)
		}
	}
	if st.Selected != "" && !validRel(st.Selected) {
		return fmt.Errorf("%w: selected path %q", ErrInvalidSidePane, st.Selected)
	}
	return nil
}

// validRel reports whether p is a clean path below a folder: not empty,
// not absolute, and with no ".", "..", or empty part.
func validRel(p string) bool {
	if p == "" || path.IsAbs(p) || strings.Contains(p, `\`) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
