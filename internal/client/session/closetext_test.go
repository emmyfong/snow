package session

import "testing"

func TestTerminalCloseText(t *testing.T) {
	tests := []struct {
		name, profile, folder, want string
	}{
		{"both", "Ubuntu", "/src", "Close this terminal (Ubuntu, /src)? Its program stops."},
		{"profile only", "Ubuntu", "", "Close this terminal (Ubuntu)? Its program stops."},
		{"folder only", "", "/src", "Close this terminal (/src)? Its program stops."},
		{"neither", "", "", "Close this terminal? Its program stops."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := terminalCloseText(tt.profile, tt.folder); got != tt.want {
				t.Fatalf("terminalCloseText(%q, %q) = %q, want %q", tt.profile, tt.folder, got, tt.want)
			}
		})
	}
}
