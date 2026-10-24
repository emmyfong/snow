package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type editTool struct{}

func init() { register(editTool{}) }

func (editTool) Def() Def {
	return Def{
		Name: "edit",
		Description: "Replace text in a file. old must appear in the file exactly once, " +
			"with its indentation; include more lines around it to make it unique.",
		Schema: schema(map[string]any{
			"path": prop("string", "the file, relative to the conversation folder or absolute"),
			"old":  prop("string", "the exact text to replace"),
			"new":  prop("string", "the text to put in its place"),
		}, "path", "old", "new"),
		Changes: true,
	}
}

func (editTool) Run(_ context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p.OS)
	if err != nil {
		return "", fmt.Errorf("edit %s: %w", in.Path, err)
	}
	after, err := replaceOnce(string(data), in.Old, in.New)
	if err != nil {
		return "", fmt.Errorf("edit %s: %w", in.Path, err)
	}
	if err := writeKeepMode(p.OS, []byte(after)); err != nil {
		return "", fmt.Errorf("edit %s: %w", in.Path, err)
	}
	added, removed := lineChanges(string(data), after)
	return fmt.Sprintf("edited %s: +%d -%d lines\n", in.Path, added, removed), nil
}

// Errors of replaceOnce.
var (
	errNoMatch   = errors.New("old text not found")
	errManyMatch = errors.New("old text appears more than once")
	errEmptyOld  = errors.New("old text is empty")
	errSameAsOld = errors.New("new text is the same as old text")
)

// replaceOnce replaces the one copy of old in s. Models write LF line
// endings, so in a CRLF file old and new get CRLF endings when only that
// makes old match.
func replaceOnce(s, old, repl string) (string, error) {
	switch old {
	case "":
		return "", errEmptyOld
	case repl:
		return "", errSameAsOld
	}
	if !strings.Contains(s, old) && strings.Contains(s, "\r\n") && !strings.Contains(old, "\r") {
		old = strings.ReplaceAll(old, "\n", "\r\n")
		repl = strings.ReplaceAll(repl, "\n", "\r\n")
	}
	switch strings.Count(s, old) {
	case 0:
		return "", errNoMatch
	case 1:
		return strings.Replace(s, old, repl, 1), nil
	default:
		return "", errManyMatch
	}
}

// lineChanges counts the lines added and removed between before and
// after, outside the lines both share at the start and the end.
func lineChanges(before, after string) (added, removed int) {
	a, b := strings.SplitAfter(before, "\n"), strings.SplitAfter(after, "\n")
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	return len(b), len(a)
}
