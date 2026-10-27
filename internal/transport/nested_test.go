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

// TestPaneVars: every pane gets SNOW, and SNOW_PANE and SNOW_SOCKET for
// snow hook, which reads them without parsing SNOW.
func TestPaneVars(t *testing.T) {
	got := PaneVars(`C:\snow\snow.sock`, 7)
	want := []string{`SNOW=C:\snow\snow.sock,7`, "SNOW_PANE=7", `SNOW_SOCKET=C:\snow\snow.sock`}
	if len(got) != len(want) {
		t.Fatalf("PaneVars() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PaneVars() = %q, want %q", got, want)
		}
	}
}
