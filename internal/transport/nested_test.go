package transport

import "testing"

func TestPaneEnvRoundTrip(t *testing.T) {
	tests := []struct {
		socket string
		pane   int
	}{
		{"/run/user/1000/snow/default.sock", 7},
		{`C:\Users\emmy\AppData\Local\snow\snow.sock`, 1},
		{"/tmp/a,b/snow.sock", 12},
	}
	for _, tt := range tests {
		assign := PaneEnv(tt.socket, tt.pane)
		value := assign[len(EnvName)+1:]
		socket, pane, ok := ParsePaneEnv(value)
		if assign[:len(EnvName)+1] != EnvName+"=" || !ok || socket != tt.socket || pane != tt.pane {
			t.Errorf("PaneEnv(%q, %d) = %q parsed as %q, %d, %v", tt.socket, tt.pane, assign, socket, pane, ok)
		}
	}
}

func TestParsePaneEnvRejects(t *testing.T) {
	for _, v := range []string{"", "1", "/sock", "/sock,x", "/sock,"} {
		if _, _, ok := ParsePaneEnv(v); ok {
			t.Errorf("ParsePaneEnv(%q) ok, want not ok", v)
		}
	}
}
