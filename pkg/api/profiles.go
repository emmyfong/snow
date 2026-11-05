package api

import "encoding/json"

// ProfileList names the programs a new pane or window can run, in the order
// the server detected them. The server sends it after each Attach.
type ProfileList struct {
	Profiles []string `json:"profiles"`
	Default  string   `json:"default"`
	// Agents names the profiles in Profiles that run an agent, such as
	// Claude Code, in their order. An agent-new command takes one.
	Agents []string `json:"agents,omitempty"`
	// Roles are the roles a new agent can take, from config.toml.
	Roles []RoleInfo `json:"roles,omitempty"`
}

// RoleInfo describes a role. Its instructions ask the agent for a way of
// working and enforce nothing. PermissionMode is the Claude Code
// permission mode the role starts the agent in, which Claude Code
// enforces; empty means Claude Code's own default. ReadOnly is set only
// when that mode keeps the agent from editing files and running commands:
// instructions alone never make a role read-only.
type RoleInfo struct {
	// Name identifies the role in ArgRole; Title is what a client shows.
	Name            string `json:"name"`
	Title           string `json:"title,omitempty"`
	PermissionMode  string `json:"permissionMode,omitempty"`
	HasInstructions bool   `json:"hasInstructions,omitempty"`
	ReadOnly        bool   `json:"readOnly,omitempty"`
}

// Type implements Message.
func (*ProfileList) Type() MsgType { return TypeProfileList }
func (*ProfileList) message()      {}

// MarshalJSON writes Profiles as [] when it is empty.
func (l ProfileList) MarshalJSON() ([]byte, error) {
	type wire ProfileList
	if l.Profiles == nil {
		l.Profiles = []string{}
	}
	return json.Marshal(wire(l))
}
