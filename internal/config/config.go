// Package config loads the user's settings from ~/.snow/config.toml. Both the
// client and the server read it, so it imports no other Snow package.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds every user setting. The struct tags name the TOML tables and
// keys, for example [keys] prefix and [scrollback] lines.
type Config struct {
	Keys       Keys       `toml:"keys"`
	Scrollback Scrollback `toml:"scrollback"`
	Profile    Profile    `toml:"profile"`
	Theme      Theme      `toml:"theme"`
}

// Keys holds the prefix key and the user's key bindings. Values stay strings
// here; the client's key table parses them.
type Keys struct {
	// Prefix is the key that starts every binding, in tmux notation, for
	// example "C-b".
	Prefix string `toml:"prefix"`
	// Bind maps a key to an action name, as in tmux, for example "|" to
	// "split-right". Several keys may name one action. The action "none"
	// unbinds a key. It holds only the user's changes; nil means no changes.
	Bind map[string]string `toml:"bind"`
}

// Scrollback sets how much history each pane keeps.
type Scrollback struct {
	// Lines is the number of lines kept above the screen. It must be at
	// least 1.
	Lines int `toml:"lines"`
}

// Profile sets which shell new panes start with.
type Profile struct {
	// Default is the name of the shell profile to preselect, for example
	// "Ubuntu". Empty means the first profile Snow detects.
	Default string `toml:"default"`
}

// Theme selects the colors.
type Theme struct {
	// Name is the theme to use, for example "default".
	Name string `toml:"name"`
}

// Defaults returns the settings Snow uses when the config file does not set
// them. Each call returns a new value, so callers may change it freely.
func Defaults() Config {
	return Config{
		Keys: Keys{Prefix: "C-b"},
		// Matches internal/term's default; see ADR 0006.
		Scrollback: Scrollback{Lines: 2000},
		Theme:      Theme{Name: "default"},
	}
}

// DefaultPath returns the config file's usual location,
// <home>/.snow/config.toml, on every OS.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: find home folder: %w", err)
	}
	return filepath.Join(home, ".snow", "config.toml"), nil
}

// Load reads the config file at path and merges it over Defaults: keys the
// file leaves out keep their default value. A missing file gives Defaults
// and no error. Any other problem returns a *Error that names the file and,
// when known, the line and the key.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, &Error{Path: path, Msg: fmt.Sprintf("cannot read the file: %v", err), Err: err}
	}
	cfg, perr := parse(string(data))
	if perr != nil {
		perr.Path = path
		return Config{}, perr
	}
	return cfg, nil
}

// parse decodes a config file's text over Defaults and checks it. The
// caller fills in the error's Path.
func parse(data string) (Config, *Error) {
	// Decoding into a filled struct sets only the keys the file holds. That
	// is the whole merge over the defaults.
	cfg := Defaults()
	md, err := toml.Decode(data, &cfg)
	if err != nil {
		var pe toml.ParseError
		if errors.As(err, &pe) {
			return Config{}, &Error{Line: pe.Position.Line, Key: pe.LastKey, Msg: pe.Message, Err: err}
		}
		// Type mismatches arrive as plain errors whose text already names
		// the line and the key.
		return Config{}, &Error{Msg: err.Error(), Err: err}
	}
	if keys := md.Undecoded(); len(keys) > 0 {
		k := keys[0]
		return Config{}, &Error{Line: keyLine(data, k), Key: k.String(), Msg: "unknown key"}
	}
	if verr := cfg.validate(); verr != nil {
		// validate names only bare keys, so splitting on dots is exact.
		verr.Line = keyLine(data, strings.Split(verr.Key, "."))
		return Config{}, verr
	}
	if verr, path := cfg.Keys.validateBind(); verr != nil {
		verr.Line = keyLine(data, path)
		return Config{}, verr
	}
	return cfg, nil
}

// validate rejects values that can never work. It leaves values that need
// parsing, such as key names, to the packages that use them.
func (c Config) validate() *Error {
	switch {
	case c.Keys.Prefix == "":
		return &Error{Key: "keys.prefix", Msg: "must not be empty"}
	case c.Scrollback.Lines < 1:
		return &Error{Key: "scrollback.lines", Msg: fmt.Sprintf("must be at least 1, got %d", c.Scrollback.Lines)}
	case c.Theme.Name == "":
		return &Error{Key: "theme.name", Msg: "must not be empty"}
	}
	return nil
}

// UnbindAction is the action that removes a key's binding.
const UnbindAction = "none"

// validateBind rejects bindings with an empty key or action. It returns the
// error and the key's path in the file, because a bound key such as "." may
// itself hold a dot. Whether the key and the action exist is for the
// client's key table to decide. The first problem in key order wins, so
// the answer does not change between runs.
func (k Keys) validateBind() (*Error, []string) {
	keys := make([]string, 0, len(k.Bind))
	for key := range k.Bind {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		path := []string{"keys", "bind", key}
		name := "keys.bind." + quoteKey(key)
		switch {
		case key == "":
			return &Error{Key: "keys.bind.\"\"", Msg: "the key must not be empty"}, path
		case k.Bind[key] == "":
			return &Error{Key: name, Msg: fmt.Sprintf("the action must not be empty; use %q to unbind", UnbindAction)}, path
		}
	}
	return nil, nil
}

// quoteKey writes a TOML key, quoted unless it is a bare key.
func quoteKey(key string) string {
	for _, r := range key {
		bare := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !bare {
			return strconv.Quote(key)
		}
	}
	return key
}
