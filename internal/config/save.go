package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrCannotEdit means a setting is written in a form Snow does not rewrite,
// such as an inline table; the user changes it by hand.
var ErrCannotEdit = errors.New("config: the setting is in a form snow cannot edit")

// SetTheme writes name as [theme] name in the config file at path, creating
// the file when it is missing. Only that one line changes: other settings,
// comments, and a comment after the value stay as they are.
func SetTheme(path, name string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	text, err := setKey(string(data), []string{"theme", "name"}, strconv.Quote(name))
	if err != nil {
		return fmt.Errorf("config: %s: theme.name: %w", path, err)
	}
	if cfg, perr := parse(text); perr != nil || cfg.Theme.Name != name {
		return fmt.Errorf("config: %s: theme.name: %w", path, ErrCannotEdit)
	}
	return writeFile(path, text)
}

// SetChatModel writes model as [chat] model and effort as [chat] effort in
// the config file at path, creating the file when it is missing. Those are
// the model and effort new chats start with. Only those two lines change:
// other settings, comments, and a comment after a value stay as they are.
func SetChatModel(path, model, effort string) error {
	if !modelName(model) {
		return fmt.Errorf("config: chat.model: must be provider/model, got %q", model)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	if dottedTable(string(data), "chat") && keyLine(string(data), []string{"chat"}) == 0 {
		// A top-level dotted key makes the table, and adding a [chat]
		// header after it would make a file other TOML readers reject.
		return fmt.Errorf("config: %s: chat.model: %w", path, ErrCannotEdit)
	}
	text, err := setKey(string(data), []string{"chat", "model"}, strconv.Quote(model))
	if err != nil {
		return fmt.Errorf("config: %s: chat.model: %w", path, err)
	}
	if text, err = setKey(text, []string{"chat", "effort"}, strconv.Quote(effort)); err != nil {
		return fmt.Errorf("config: %s: chat.effort: %w", path, err)
	}
	if cfg, perr := parse(text); perr != nil {
		return fmt.Errorf("config: %s: chat.model: %w", path, ErrCannotEdit)
	} else if cfg.Chat.Model != model {
		return fmt.Errorf("config: %s: chat.model: %w", path, ErrCannotEdit)
	} else if cfg.Chat.Effort != effort {
		return fmt.Errorf("config: %s: chat.effort: %w", path, ErrCannotEdit)
	}
	return writeFile(path, text)
}

// dottedTable reports whether a top-level dotted key starts the table name,
// such as chat.model = "..." for chat.
func dottedTable(data, name string) bool {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '[' {
			continue
		}
		parts, rest := splitKey(line)
		if len(parts) > 1 && parts[0] == name && strings.HasPrefix(rest, "=") {
			return true
		}
	}
	return false
}

// setKey sets a two-part key ([table] key) to value, a TOML literal, in a
// document. It replaces the value on the key's line, adds the key under the
// table's header, or appends the table.
func setKey(data string, key []string, value string) (string, error) {
	lines := strings.Split(data, "\n")
	n := keyLine(data, key)
	if n == 0 {
		text := strings.TrimRight(data, "\n")
		if text != "" {
			text += "\n\n"
		}
		return text + "[" + key[0] + "]\n" + key[1] + " = " + value + "\n", nil
	}
	line := lines[n-1]
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "[") {
		lines = append(lines[:n], append([]string{key[1] + " = " + value}, lines[n:]...)...)
		return strings.Join(lines, "\n"), nil
	}
	parts, rest := splitKey(trimmed)
	if len(parts) == 0 || parts[len(parts)-1] != key[1] || !strings.HasPrefix(rest, "=") {
		return "", ErrCannotEdit // the parent is an inline table
	}
	eq := strings.Index(line, "=")
	after := strings.TrimLeft(line[eq+1:], " \t")
	end := valueEnd(after)
	if end < 0 {
		return "", ErrCannotEdit
	}
	lines[n-1] = strings.TrimRight(line[:eq], " \t") + " = " + value + after[end:]
	return strings.Join(lines, "\n"), nil
}

// valueEnd returns the index just past the string value at the start of s,
// or -1 when s does not start with a one-line string.
func valueEnd(s string) int {
	switch {
	case strings.HasPrefix(s, `"""`), strings.HasPrefix(s, "'''"):
		return -1
	case strings.HasPrefix(s, `"`):
		if end := closingQuote(s); end > 0 {
			return end + 1
		}
	case strings.HasPrefix(s, "'"):
		if end := strings.IndexByte(s[1:], '\''); end >= 0 {
			return end + 2
		}
	}
	return -1
}

// writeFile replaces path with text: it writes a temporary file beside it
// and renames it over, so a failed write never leaves half a config.
func writeFile(path, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	_, werr := tmp.WriteString(text)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}
