package sessionlist

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Sort returns a copy of sessions, most recently used first; sessions used
// at the same time keep name order.
func Sort(sessions []api.SessionInfo) []api.SessionInfo {
	out := slices.Clone(sessions)
	slices.SortFunc(out, func(a, b api.SessionInfo) int {
		return cmp.Or(b.LastUsed.Compare(a.LastUsed), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// Items returns one list row per session: its name, its state, and when it
// was last used, as seen at now, with an alert when agents in it wait for
// the user, marked with g's needs-you glyph.
func Items(sessions []api.SessionInfo, now time.Time, g theme.Glyphs) []ui.ListItem {
	items := make([]ui.ListItem, len(sessions))
	for i, s := range sessions {
		items[i] = ui.ListItem{Cols: []string{s.Name, State(s), Ago(now.Sub(s.LastUsed))}, Alert: needsYou(s, g)}
	}
	return items
}

// needsYou says that agents in s wait for the user, in words, so color is
// not the only sign; "" when none waits.
func needsYou(s api.SessionInfo, g theme.Glyphs) string {
	switch {
	case s.NeedsYou <= 0:
		return ""
	case s.NeedsYou == 1:
		return g.AgentNeedsYou + " needs you"
	}
	return g.AgentNeedsYou + " needs you (" + strconv.Itoa(s.NeedsYou) + ")"
}

// State describes a session in a few words: running or saved, and how many
// clients show it.
func State(s api.SessionInfo) string {
	switch s.Attached {
	case 0:
		return s.State
	case 1:
		return s.State + ", 1 client"
	default:
		return s.State + ", " + strconv.Itoa(s.Attached) + " clients"
	}
}

// Ago says how long ago something happened, in the words a person uses.
func Ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return strconv.Itoa(int(d.Hours()/24)) + " days ago"
	}
}
