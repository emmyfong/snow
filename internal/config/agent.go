package config

import (
	"fmt"
	"slices"
	"strings"
)

// Agent is an agent profile the user adds in an [agent.<name>] table: a
// program for a pane, with its own environment, for example Claude Code
// pointed at a local gateway. Each profile is a choice in the pane picker.
type Agent struct {
	// Title is the name the picker shows and that saved sessions keep.
	// Empty means the table's name.
	Title string `toml:"title"`
	// Command is the program and its arguments, for example ["claude"].
	Command []string `toml:"command"`
	// Env holds variables the pane gets. A value "$secret:<name>" is read
	// from the credential store when the pane starts, so tokens stay out
	// of this file; `snow auth login <name>` saves it.
	Env map[string]string `toml:"env"`
}

// SecretPrefix starts an Env value that names a saved secret.
const SecretPrefix = "$secret:"

// SecretName returns the secret an Env value names, and whether it names
// one. Only a whole value names a secret.
func SecretName(value string) (string, bool) {
	return strings.CutPrefix(value, SecretPrefix)
}

// TitleOf returns the profile's title: Title, or name when it has none.
func (a Agent) TitleOf(name string) string {
	if t := strings.TrimSpace(a.Title); t != "" {
		return t
	}
	return name
}

// Variables an agent profile must not set: the server sets them in every
// pane, and snow hook relies on them.
const reservedPrefix = "SNOW"

// secretSuffixes end the names of variables that hold secrets. Their value
// must come from the credential store, never from the file.
var secretSuffixes = []string{"_KEY", "_TOKEN", "_SECRET", "_PASSWORD"}

// validateAgents rejects agent profiles that can never work. It returns
// the error and the setting's path in the file. The first problem in name
// order wins, so the answer does not change between runs.
func (c Config) validateAgents() (*Error, []string) {
	names := make([]string, 0, len(c.Agents))
	for name := range c.Agents {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if e, path := validateAgent(name, c.Agents[name]); e != nil {
			return e, path
		}
	}
	return nil, nil
}

func validateAgent(name string, a Agent) (*Error, []string) {
	key := "agent." + quoteKey(name)
	if msg := providerName(name); msg != "" {
		return &Error{Key: key, Msg: msg}, []string{"agent", name}
	}
	if len(a.Command) == 0 || strings.TrimSpace(a.Command[0]) == "" {
		return &Error{Key: key + ".command", Msg: `must name a program, for example ["claude"]`},
			[]string{"agent", name, "command"}
	}
	vars := make([]string, 0, len(a.Env))
	for v := range a.Env {
		vars = append(vars, v)
	}
	slices.Sort(vars)
	for _, v := range vars {
		path := []string{"agent", name, "env", v}
		vkey := key + ".env." + quoteKey(v)
		if msg := envVar(v, a.Env[v]); msg != "" {
			return &Error{Key: vkey, Msg: msg}, path
		}
	}
	if a.Env["ANTHROPIC_BASE_URL"] != "" && a.Env["ANTHROPIC_AUTH_TOKEN"] == "" && a.Env["ANTHROPIC_API_KEY"] == "" {
		// Without a token of its own, Claude Code would send the user's
		// own login to the other host.
		return &Error{Key: key + ".env.ANTHROPIC_BASE_URL",
				Msg: `needs ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY too, for example "$secret:<name>"`},
			[]string{"agent", name, "env", "ANTHROPIC_BASE_URL"}
	}
	return nil, nil
}

// envVar says what is wrong with an environment variable of a profile, or
// "".
func envVar(name, value string) string {
	ok := name != "" && (name[0] < '0' || name[0] > '9')
	for _, r := range name {
		word := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_'
		if !word {
			ok = false
		}
	}
	upper := strings.ToUpper(name)
	switch {
	case !ok:
		return "a variable name must be letters, digits, and underscores, and not start with a digit"
	case strings.HasPrefix(upper, reservedPrefix):
		return "Snow sets the SNOW variables itself"
	}
	if secret, isSecret := SecretName(value); isSecret {
		if msg := providerName(secret); msg != "" {
			return "names a bad secret: " + msg
		}
		return ""
	}
	if value != "" && slices.ContainsFunc(secretSuffixes, func(s string) bool { return strings.HasSuffix(upper, s) }) {
		return fmt.Sprintf(`must not hold a secret in the file; use "%s<name>" and save it with snow auth login <name>`, SecretPrefix)
	}
	return ""
}
