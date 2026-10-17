package home

import (
	"strconv"
	"time"
)

// ago says how long ago something happened, in the words a person uses.
func ago(d time.Duration) string {
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
