package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSessionRequestsValidate(t *testing.T) {
	tests := []struct {
		name string
		v    interface{ Validate() error }
		ok   bool
	}{
		{"kill", &KillSession{Session: "api"}, true},
		{"kill with no session", &KillSession{}, false},
		{"rename", &RenameSession{Session: "api", Name: "web app"}, true},
		{"rename to the longest name", &RenameSession{Session: "api", Name: strings.Repeat("a", MaxSessionName)}, true},
		{"rename with no session", &RenameSession{Name: "web"}, false},
		{"rename to nothing", &RenameSession{Session: "api"}, false},
		{"rename to a long name", &RenameSession{Session: "api", Name: strings.Repeat("a", MaxSessionName+1)}, false},
		{"rename with spaces around", &RenameSession{Session: "api", Name: " web"}, false},
		{"rename with a control character", &RenameSession{Session: "api", Name: "we\tb"}, false},
		{"rename to bad UTF-8", &RenameSession{Session: "api", Name: "\xff"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.v.Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidSessionRequest) {
				t.Fatalf("error %v does not wrap ErrInvalidSessionRequest", err)
			}
		})
	}
}
