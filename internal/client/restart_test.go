package client

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestAskRestart(t *testing.T) {
	m := &api.VersionMismatchError{ClientVersion: "v0.2.0", ServerVersion: "v0.1.0", ClientProtocol: 5, ServerProtocol: 4}
	for answer, want := range map[string]bool{"y\n": true, "yes\n": true, "Y\n": true, "\n": false, "n\n": false, "": false} {
		var out bytes.Buffer
		if got := AskRestart(strings.NewReader(answer), &out, m); got != want {
			t.Errorf("answer %q: restart %v, want %v", answer, got, want)
		}
		if !strings.Contains(out.String(), "v0.1.0") || !strings.Contains(out.String(), "saved") {
			t.Errorf("question %q should name the old version and say sessions are saved", out.String())
		}
	}
}
