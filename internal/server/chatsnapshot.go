package server

import (
	"encoding/json"

	"github.com/emmyfong/snow/pkg/api"
)

// snapshotBudget is how large an encoded snapshot may be: a frame, less
// room for growth between the check and the write.
const snapshotBudget = api.MaxFrameSize * 3 / 4

// fitSnapshot returns a copy of snap whose encoding fits snapshotBudget,
// leaving out its oldest entries if needed.
func fitSnapshot(snap api.ChatSnapshot) *api.ChatSnapshot {
	out := snap
	out.Entries = nil
	base, err := json.Marshal(out)
	if err != nil {
		return &out
	}
	size, keep := len(base), len(snap.Entries)
	for keep > 0 {
		b, err := json.Marshal(snap.Entries[keep-1])
		if err != nil || size+len(b)+1 > snapshotBudget {
			break
		}
		size += len(b) + 1
		keep--
	}
	out.Earlier = snap.Earlier + keep
	out.Entries = append([]api.ChatEntry(nil), snap.Entries[keep:]...)
	return &out
}
