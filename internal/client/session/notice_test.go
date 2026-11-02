package session

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestNoticeIsNotShownAsError: a notice takes the notice slot of the
// status bar, not the slot for a refused request (#238).
func TestNoticeIsNotShownAsError(t *testing.T) {
	m := New(func(api.Message) {})
	m.Update(&api.Error{Code: api.CodeNotice, Message: "started in ask mode"})
	if m.notice != "started in ask mode" || m.message != "" {
		t.Fatalf("notice %q, message %q", m.notice, m.message)
	}
	m.Update(&api.Error{Code: api.CodeCommandFailed, Message: "no"})
	if m.message != "no" {
		t.Fatalf("a refusal is not in the message slot: %q", m.message)
	}
}
