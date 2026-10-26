package chat

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestConversationCommands: each command sends one input, asks the session
// for an overlay, or says what to type. Nothing else reaches the server.
func TestConversationCommands(t *testing.T) {
	tests := []struct {
		name, text string
		effort     string // the chat's effort
		kind       string // the input sent, or ""
		check      func(*api.ChatInput) bool
		ask        string // the overlay asked for, or ""
		note       string // text the note holds, or ""
	}{
		{name: "new", text: "/new", kind: api.ChatInputNew},
		{name: "resume", text: "/resume", ask: AskResume},
		{name: "rename", text: "/rename  Fix the bug ", kind: api.ChatInputRename, check: func(in *api.ChatInput) bool { return in.Text == "Fix the bug" }},
		{name: "rename without a title", text: "/rename", note: "new title"},
		{name: "rename too long", text: "/rename " + strings.Repeat("x", api.MaxChatTitle+1), note: "at most"},
		{name: "mode picker", text: "/mode", ask: AskPickMode},
		{name: "mode auto", text: "/mode auto", kind: api.ChatInputMode, check: func(in *api.ChatInput) bool { return in.Mode == api.ChatModeAuto }},
		{name: "mode ask in capitals", text: "/mode ASK", kind: api.ChatInputMode, check: func(in *api.ChatInput) bool { return in.Mode == api.ChatModeAsk }},
		{name: "mode bad word", text: "/mode yolo", note: "ask or auto"},
		{name: "effort level", text: "/effort high", kind: api.ChatInputEffort, check: func(in *api.ChatInput) bool { return in.Effort == "high" }},
		{name: "effort default", text: "/effort default", effort: "high", kind: api.ChatInputEffort, check: func(in *api.ChatInput) bool { return in.Effort == "" }},
		{name: "effort bad word", text: "/effort loud", note: `Unknown effort "loud"`},
		{name: "effort shows the current one", text: "/effort", effort: "medium", note: "Effort is medium"},
		{name: "help", text: "/help", ask: AskHelp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newChat(100, 24)
			m.snap.Effort = tt.effort
			in := runSlash(m, tt.text)
			switch {
			case tt.kind == "" && in != nil:
				t.Fatalf("%s sent %+v", tt.text, in)
			case tt.kind != "" && (in == nil || in.Kind != tt.kind || in.Pane != 2 || in.Agent != "c1"):
				t.Fatalf("%s sent %+v, want kind %s", tt.text, in, tt.kind)
			case tt.check != nil && !tt.check(in):
				t.Fatalf("%s sent %+v", tt.text, in)
			}
			if got := m.Asked(); got != tt.ask {
				t.Fatalf("Asked = %q, want %q", got, tt.ask)
			}
			if note := m.Note(); !strings.Contains(note, tt.note) {
				t.Fatalf("note = %q, want it to hold %q", note, tt.note)
			}
		})
	}
}

// TestEffortNoteListsTheLevels: a bad level names every level a user can type.
func TestEffortNoteListsTheLevels(t *testing.T) {
	m, _ := newChat(100, 24)
	runSlash(m, "/effort loud")
	note := m.Note() // reading it clears it
	for _, l := range api.EffortLevels {
		if !strings.Contains(note, l) {
			t.Fatalf("note %q lacks %q", note, l)
		}
	}
}

// TestCommandsAreInTheChatList: the new commands show in the chat's list.
func TestCommandsAreInTheChatList(t *testing.T) {
	m, _ := newChat(100, 40)
	typeText(m, "/")
	got := screen(m)
	for _, name := range []string{"/new", "/resume", "/rename", "/mode", "/effort", "/help"} {
		if !strings.Contains(got, name) {
			t.Errorf("the list lacks %s:\n%s", name, got)
		}
	}
}
