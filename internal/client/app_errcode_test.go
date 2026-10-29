package client

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestNoticeKeepsClient: a notice is information, so it does not end the
// client, and the status bar shows it (#238).
func TestNoticeKeepsClient(t *testing.T) {
	a := newAuthApp()
	const text = "Resumed in /work with the approval mode ask."
	a.Update(&api.Error{Code: api.CodeNotice, Message: text})
	if a.result.Err != nil {
		t.Fatalf("a notice ended the client: %v", a.result.Err)
	}
	if s := a.screenText(); !strings.Contains(s, text) {
		t.Fatalf("the status bar does not show the notice:\n%s", s)
	}
}

// TestNotTerminalKeepsClient: a scrollback request for a chat pane is
// refused, and the client shows why (#238).
func TestNotTerminalKeepsClient(t *testing.T) {
	a := newAuthApp()
	const text = "pane 2 is a chat pane; it has no screen"
	a.Update(&api.Error{Code: api.CodeNotTerminal, Message: text})
	if a.result.Err != nil {
		t.Fatalf("a refused scrollback request ended the client: %v", a.result.Err)
	}
	if s := a.screenText(); !strings.Contains(s, text) {
		t.Fatalf("the status bar does not show the reason:\n%s", s)
	}
}

// TestErrorCodesEndClient: only the codes that leave the client with no
// session to show end it. Any other code, including one this client does
// not know, goes to the screen.
func TestErrorCodesEndClient(t *testing.T) {
	for _, tc := range []struct {
		code string
		ends bool
	}{
		{api.CodeBadHandshake, true},
		{api.CodeNoSession, true},
		{api.CodeAttachFailed, true},
		{api.CodeBadSize, false},
		{api.CodeBadCommand, false},
		{api.CodeCommandFailed, false},
		{api.CodeNotTerminal, false},
		{api.CodeAuthFailed, false},
		{api.CodeNotice, false},
		{"a-code-from-a-newer-server", false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			a := newAuthApp()
			a.Update(&api.Error{Code: tc.code, Message: "why"})
			if got := a.result.Err != nil; got != tc.ends {
				t.Fatalf("ends the client = %v, want %v", got, tc.ends)
			}
		})
	}
}
