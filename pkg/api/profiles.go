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
