package agents

import (
	"strconv"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// kindName names an agent kind for people.
func kindName(kind string) string {
	switch kind {
	case api.AgentKindClaudeCode:
		return "Claude Code"
	case api.AgentKindChat:
		return "chat"
	}
	return kind
}

// stateText is an agent state as its glyph and a word, so color is not
// the only sign.
func stateText(state string, g theme.Glyphs) string {
	switch state {
	case api.AgentIdle:
		return g.AgentIdle + " idle"
	case api.AgentWorking:
		return g.AgentWorking + " working"
	case api.AgentNeedsYou:
		return g.AgentNeedsYou + " needs you"
	}
	return "starting"
}

// waitText says what an agent that needs the user waits for.
func waitText(a api.AgentInfo) string {
	switch a.Waits {
	case api.AgentWaitsApproval:
		return "approval"
	case api.AgentWaitsQuestion:
		return "question"
	}
	return "waiting"
}

// place is where an agent is: its window's index and name.
func place(a api.AgentInfo) string {
	return strings.TrimSpace(strconv.Itoa(a.Window) + " " + a.WindowName)
}

// span says briefly how long an agent has been in its state: 12s, 3m, 2h,
// 4d. A time ahead of now counts as 0s.
func span(d time.Duration) string {
	switch {
	case d < time.Minute:
		return strconv.Itoa(int(max(d, 0).Seconds())) + "s"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d"
}

// folderName is the last element of a folder path, on any OS: the server
// may run on another one than the client, such as in WSL.
func folderName(path string) string {
	path = strings.TrimRight(path, `/\`)
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}
