package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Role is an agent role the user adds in a [role.<name>] table. The new
// agent form offers it. Instructions are text the agent gets after its
// own; they ask, and enforce nothing. PermissionMode is a Claude Code
// permission mode, which Claude Code enforces: "plan" keeps the agent
// from editing files and running commands.
type Role struct {
	// Title is the name the form shows. Empty means the table's name.
	Title string `toml:"title"`
	// Instructions are added to the agent's system prompt.
	Instructions string `toml:"instructions"`
	// PermissionMode is one of PermissionModes, or empty for Claude Code's
	// own default.
	PermissionMode string `toml:"permission_mode"`
}

// PermissionModes are the Claude Code permission modes a role may set.
// bypassPermissions is left out: a role must not turn off every check.
var PermissionModes = []string{"acceptEdits", "auto", "dontAsk", "manual", "plan"}

// MaxInstructions bounds a role's instructions, which every launch with the
// role writes to a file.
const MaxInstructions = 64 << 10

// TitleOf returns the role's title: Title, or name when it has none.
func (r Role) TitleOf(name string) string {
	if t := strings.TrimSpace(r.Title); t != "" {
		return t
	}
	return name
}

// validateRoles rejects roles that can never work, the first in name
// order, with the setting's path in the file.
func (c Config) validateRoles() (*Error, []string) {
	for _, name := range slices.Sorted(maps.Keys(c.Roles)) {
		r := c.Roles[name]
		key := "role." + quoteKey(name)
		switch {
		case strings.TrimSpace(r.Instructions) == "" && r.PermissionMode == "":
			return &Error{Key: key, Msg: "must set instructions, permission_mode, or both"}, []string{"role", name}
		case len(r.Instructions) > MaxInstructions:
			return &Error{Key: key + ".instructions", Msg: fmt.Sprintf("must be at most %d bytes", MaxInstructions)},
				[]string{"role", name, "instructions"}
		case r.PermissionMode != "" && !slices.Contains(PermissionModes, r.PermissionMode):
			return &Error{Key: key + ".permission_mode", Msg: "must be one of " + strings.Join(PermissionModes, ", ")},
				[]string{"role", name, "permission_mode"}
		}
	}
	return nil, nil
}
