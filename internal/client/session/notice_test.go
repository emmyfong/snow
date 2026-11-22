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

func TestNoticePreservesPendingShellStart(t *testing.T) {
	m := New(func(api.Message) {})
	m.pendingShell = true
	m.shellStart = 2
	m.early = []api.Input{{Pane: 1}}
	m.Update(&api.Error{Code: api.CodeNotice, Message: "folder started in home"})
	if !m.pendingShell || m.shellStart != 2 || len(m.early) != 1 || m.notice != "folder started in home" {
		t.Fatalf("notice changed pending shell start: pending=%v phase=%d early=%v notice=%q", m.pendingShell, m.shellStart, m.early, m.notice)
	}
}

func TestNoticePreservesPendingNewWindow(t *testing.T) {
	m := New(func(api.Message) {})
	m.newWindowPending = true
	m.newPreset = "ide"
	m.pendingWindowRuntime = "runtime-1"
	m.Update(&api.Error{Code: api.CodeNotice, Message: "folder started in home"})
	if !m.newWindowPending || m.newPreset != "ide" || m.pendingWindowRuntime != "runtime-1" || m.notice != "folder started in home" {
		t.Fatalf("notice changed pending new window: pending=%v preset=%q runtime=%q notice=%q", m.newWindowPending, m.newPreset, m.pendingWindowRuntime, m.notice)
	}
}
