package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
)

// Limits of the list tool.
const (
	listEntries = 1000
	listBytes   = 64 << 10
)

type listTool struct{}

func init() { register(listTool{}) }

func (listTool) Def() Def {
	return Def{
		Name: "list",
		Description: "List the files and folders under a folder, one relative path per line; " +
			"folders end with /. What .gitignore ignores is left out.",
		Schema: schema(map[string]any{
			"path": prop("string", "the folder, relative to the conversation folder or absolute; default the conversation folder"),
		}),
	}
}

func (listTool) Run(ctx context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	if in.Path == "" {
		in.Path = "."
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(p.OS); err != nil {
		return "", fmt.Errorf("list %s: %w", in.Path, err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("%s is a file; use read", in.Path)
	}
	out := capped{limit: listBytes}
	n, more := 0, 0
	err = walk(ctx, p.OS, func(rel string, d fs.DirEntry) error {
		if d.IsDir() {
			rel += "/"
		}
		if n >= listEntries || !out.add(rel) {
			more++
			return nil
		}
		n++
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("list %s: %w", in.Path, err)
	}
	if n == 0 {
		return "(empty folder)\n", nil
	}
	if more > 0 {
		out.full = true
		return out.String(fmt.Sprintf("%d more entries not shown; list a subfolder", more)), nil
	}
	return out.String(""), nil
}
