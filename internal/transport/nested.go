package transport

import (
	"strconv"
	"strings"
)

// EnvName is the variable the server sets in every pane, like tmux's TMUX.
// A snow that finds it set is running inside a snow pane.
const EnvName = "SNOW"

// EnvPane and EnvSocket are the variables the server sets in every pane for
// snow hook: the pane's ID and the server's socket path.
const (
	EnvPane   = "SNOW_PANE"
	EnvSocket = "SNOW_SOCKET"
)

// PaneVars returns every variable the server sets in pane id of the server
// at socket: EnvName, EnvPane, and EnvSocket.
func PaneVars(socket string, pane int) []string {
	return []string{PaneEnv(socket, pane), EnvPane + "=" + strconv.Itoa(pane), EnvSocket + "=" + socket}
}

// PaneEnv returns the EnvName assignment for pane id of the server at socket:
// the socket path, a comma, and the pane ID.
func PaneEnv(socket string, pane int) string {
	return EnvName + "=" + socket + "," + strconv.Itoa(pane)
}

// ParsePaneEnv splits an EnvName value into the socket path and the pane ID.
// ok is false when value is not in the form PaneEnv writes. The split is at
// the last comma because a socket path may contain commas.
func ParsePaneEnv(value string) (socket string, pane int, ok bool) {
	i := strings.LastIndexByte(value, ',')
	if i < 0 {
		return "", 0, false
	}
	pane, err := strconv.Atoi(value[i+1:])
	if err != nil {
		return "", 0, false
	}
	return value[:i], pane, true
}
