package client

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	// An error shows the same text in its own strip, so the bar must differ.
	e := newAuthApp()
	e.Update(&api.Error{Code: api.CodeCommandFailed, Message: text})
	if a.View().Content == e.View().Content {
		t.Fatal("the notice is drawn like an error")
	}
}

// TestResumeNoticeFits: the notice the server sends on a resume shows on an
// ordinary terminal, not only on a wide one (#238). The text is the
// server's, from internal/server/chatswap.go, with a typical folder.
func TestResumeNoticeFits(t *testing.T) {
	const text = "Resumed with the approval mode ask. Type /mode auto to stop the questions. Saved in /home/emmyf/projects/snow."
	for _, width := range []int{80, 100, 120} {
		a := newAuthApp()
		a.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		a.Update(&api.Error{Code: api.CodeNotice, Message: text})
		if s := a.screenText(); !strings.Contains(s, "Type /mode auto") {
			t.Fatalf("width %d: the status bar hides the notice:\n%s", width, s)
		}
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
		{api.CodeSessionEnded, false},
		{"a-code-from-a-newer-server", false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			a := newAuthApp()
			_, cmd := a.Update(&api.Error{Code: tc.code, Message: "why"})
			if got := a.result.Err != nil; got != tc.ends {
				t.Fatalf("ends the client = %v, want %v", got, tc.ends)
			}
			if tc.ends {
				// Only a quit is safe to call: another command may be a timer.
				if cmd == nil {
					t.Fatal("the client has an error but does not quit")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatalf("the command is not a quit: %T", cmd())
				}
			}
		})
	}
}
