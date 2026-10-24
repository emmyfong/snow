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

// writeKeepMode writes data to file. An existing file keeps its mode; a
// new one gets 0644, which the umask narrows.
func writeKeepMode(file string, data []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, data, mode)
}

// Paths implements Pather.
func (writeTool) Paths(input json.RawMessage) ([]string, error) { return pathField(input, "") }
