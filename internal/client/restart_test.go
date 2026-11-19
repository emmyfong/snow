package client

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestRestartQuestionText(t *testing.T) {
	for _, tt := range []struct{ client, server string }{{"v0.2.0", "v0.1.0"}, {"dev", "old"}} {
		t.Run(tt.client+"/"+tt.server, func(t *testing.T) {
			title, text := restartQuestion(&api.VersionMismatchError{ClientVersion: tt.client, ServerVersion: tt.server})
			if title != "Restart server?" {
				t.Errorf("title %q, want Restart server?", title)
			}
			for _, want := range []string{tt.client, tt.server, "Programs running in its panes stop", "sessions keeps them", "saved sessions"} {
				if !strings.Contains(text, want) {
					t.Errorf("question %q does not contain %q", text, want)
				}
			}
		})
	}
}
