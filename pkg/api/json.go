package api

import "encoding/json"

// Arrays are never null on the wire, so other clients can always iterate
// them. These methods turn nil slices into empty ones.

// MarshalJSON writes Windows as [] when it is empty.
func (l Layout) MarshalJSON() ([]byte, error) {
	type wire Layout
	if l.Windows == nil {
		l.Windows = []WindowInfo{}
	}
	return json.Marshal(wire(l))
}

// MarshalJSON writes Panes as [] when it is empty.
func (w WindowInfo) MarshalJSON() ([]byte, error) {
	type wire WindowInfo
	if w.Panes == nil {
		w.Panes = []PaneInfo{}
	}
	return json.Marshal(wire(w))
}

// MarshalJSON writes Lines as [] when it is empty.
func (u PaneUpdate) MarshalJSON() ([]byte, error) {
	type wire PaneUpdate
	if u.Lines == nil {
		u.Lines = []LineUpdate{}
	}
	return json.Marshal(wire(u))
}

// MarshalJSON writes Sessions as [] when it is empty.
func (l SessionList) MarshalJSON() ([]byte, error) {
	type wire SessionList
	if l.Sessions == nil {
		l.Sessions = []SessionInfo{}
	}
	return json.Marshal(wire(l))
}
