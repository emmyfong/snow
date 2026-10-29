//go:build windows || linux

package notify

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestToastTextReachesWindows: the PowerShell script reads the text from the
// environment, and WSLENV names both variables so WSL passes them to the
// Windows program. Other WSLENV entries stay.
func TestToastTextReachesWindows(t *testing.T) {
	t.Setenv("WSLENV", "EXISTING/p")
	cmd := toastCommand(context.Background(), Notice{Kind: NeedsYou, Agent: "Claude Code", Session: "api"}, "powershell.exe")
	for _, want := range []string{
		"SNOW_NOTIFY_TITLE=Snow",
		`SNOW_NOTIFY_BODY=Claude Code in session "api" needs you`,
	} {
		if !slices.Contains(cmd.Env, want) {
			t.Errorf("environment lacks %q", want)
		}
	}
	var wsl []string
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "WSLENV=") {
			wsl = append(wsl, e)
		}
	}
	if len(wsl) != 1 {
		t.Fatalf("WSLENV entries %v, want exactly one", wsl)
	}
	for _, want := range []string{"EXISTING/p", "SNOW_NOTIFY_TITLE", "SNOW_NOTIFY_BODY"} {
		if !hasWSLEntry(strings.Split(strings.TrimPrefix(wsl[0], "WSLENV="), ":"), want) {
			t.Errorf("WSLENV %q lacks %q", wsl[0], want)
		}
	}
	if cmd.Args[0] != "powershell.exe" || !slices.Contains(cmd.Args, "-Command") {
		t.Fatalf("command %v, want powershell running the toast script", cmd.Args)
	}
}

// TestWithWSLEnv: names are added once, and another program's entries keep
// their order.
func TestWithWSLEnv(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{"none", []string{"HOME=/home/u"}, "WSLENV=SNOW_NOTIFY_TITLE:SNOW_NOTIFY_BODY"},
		{"existing", []string{"WSLENV=FOO/p:BAR", "HOME=/home/u"}, "WSLENV=FOO/p:BAR:SNOW_NOTIFY_TITLE:SNOW_NOTIFY_BODY"},
		{"already listed", []string{"WSLENV=SNOW_NOTIFY_TITLE/p:SNOW_NOTIFY_BODY"}, "WSLENV=SNOW_NOTIFY_TITLE/p:SNOW_NOTIFY_BODY"},
		{"empty", []string{"WSLENV="}, "WSLENV=SNOW_NOTIFY_TITLE:SNOW_NOTIFY_BODY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withWSLEnv(tt.env, "SNOW_NOTIFY_TITLE", "SNOW_NOTIFY_BODY")
			if last := got[len(got)-1]; last != tt.want {
				t.Fatalf("WSLENV %q, want %q", last, tt.want)
			}
			for _, e := range tt.env {
				if strings.HasPrefix(e, "WSLENV=") {
					continue
				}
				if !slices.Contains(got, e) {
					t.Errorf("lost environment entry %q", e)
				}
			}
		})
	}
}
