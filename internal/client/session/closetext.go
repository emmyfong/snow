package session

import "strings"

func terminalCloseText(profile, folder string) string {
	parts := make([]string, 0, 2)
	if profile = strings.TrimSpace(profile); profile != "" {
		parts = append(parts, profile)
	}
	if folder = strings.TrimSpace(folder); folder != "" {
		parts = append(parts, folder)
	}
	if len(parts) == 0 {
		return "Close this terminal? Its program stops."
	}
	return "Close this terminal (" + strings.Join(parts, ", ") + ")? Its program stops."
}
