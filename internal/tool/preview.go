package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Change is what a call would do to one file, for the user to see before
// approving it.
type Change struct {
	Path string // as the model wrote it
	Old  string // empty for a new file
	New  string
}

// Previewer is a tool that can show its change to a file before it runs.
type Previewer interface {
	// Preview returns the change the call would make. An error means the
	// call would fail; the approval step then asks without a preview.
	Preview(env Env, input json.RawMessage) (Change, error)
}

// Preview implements Previewer.
func (editTool) Preview(env Env, raw json.RawMessage) (Change, error) {
	var in struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := input(raw, &in); err != nil {
		return Change{}, err
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return Change{}, err
	}
	data, err := os.ReadFile(p.OS)
	if err != nil {
		return Change{}, fmt.Errorf("edit %s: %w", in.Path, err)
	}
	after, err := replaceOnce(string(data), in.Old, in.New)
	if err != nil {
		return Change{}, fmt.Errorf("edit %s: %w", in.Path, err)
	}
	return Change{Path: in.Path, Old: string(data), New: after}, nil
}

// Preview implements Previewer.
func (writeTool) Preview(env Env, raw json.RawMessage) (Change, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := input(raw, &in); err != nil {
		return Change{}, err
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return Change{}, err
	}
	data, err := os.ReadFile(p.OS)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Change{}, fmt.Errorf("write %s: %w", in.Path, err)
	}
	return Change{Path: in.Path, Old: string(data), New: in.Content}, nil
}
