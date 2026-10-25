package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type writeTool struct{}

func init() { register(writeTool{}) }

func (writeTool) Def() Def {
	return Def{
		Name: "write",
		Description: "Create a file, or replace all of an existing file, with content. " +
			"Missing parent folders are created. To change part of a file, use edit.",
		Schema: schema(map[string]any{
			"path":    prop("string", "the file, relative to the conversation folder or absolute"),
			"content": prop("string", "the whole content of the file"),
		}, "path", "content"),
		Changes: true,
	}
}

func (writeTool) Run(_ context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return "", err
	}
	verb := "overwrote"
	if info, err := os.Stat(p.OS); errors.Is(err, fs.ErrNotExist) {
		verb = "created"
	} else if err == nil && info.IsDir() {
		return "", fmt.Errorf("%s is a folder", in.Path)
	}
	if err := os.MkdirAll(filepath.Dir(p.OS), 0o755); err != nil {
		return "", fmt.Errorf("write %s: %w", in.Path, err)
	}
	if err := writeKeepMode(p.OS, []byte(in.Content)); err != nil {
		return "", fmt.Errorf("write %s: %w", in.Path, err)
	}
	lines := strings.Count(in.Content, "\n")
	if in.Content != "" && !strings.HasSuffix(in.Content, "\n") {
		lines++
	}
	return fmt.Sprintf("%s %s: %d lines\n", verb, in.Path, lines), nil
}

// writeKeepMode writes data to file through a temp file in the same
// folder and a rename, so a crash or a full disk never leaves the file cut
// short. An existing file keeps its mode; a new one gets 0644, which the
// umask narrows. A link is kept: its target is written.
func writeKeepMode(file string, data []byte) error {
	if target, err := filepath.EvalSymlinks(file); err == nil {
		file = target
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".snow-write-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if werr == nil {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), mode)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), file)
	}
	if werr != nil {
		return errors.Join(werr, removeIfThere(tmp.Name()))
	}
	return nil
}

// removeIfThere removes file; a file that is gone already is not an error.
func removeIfThere(file string) error {
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Paths implements Pather.
func (writeTool) Paths(input json.RawMessage) ([]string, error) { return pathField(input, "") }
