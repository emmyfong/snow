package chat

import (
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/helppanel"
	"github.com/emmyfong/snow/pkg/api"
)

// Mode returns the chat's approval mode: "ask" or "auto".
func (m *Model) Mode() string { return m.snap.Mode }

// runNew handles /new. The server refuses it while a turn runs.
func runNew(h Host, _ string) { h.Send(api.ChatInputNew, "") }

// runResume asks the session for the picker of past conversations.
func runResume(h Host, _ string) { h.Ask(AskResume) }

// runRename handles /rename: it renames the conversation, not the session.
func runRename(h Host, rest string) {
	title := strings.TrimSpace(rest)
	switch {
	case title == "":
		h.Notice("Type /rename and the new title.")
	case len([]rune(title)) > api.MaxChatTitle:
		h.Notice(fmt.Sprintf("A title takes at most %d characters.", api.MaxChatTitle))
	default:
		h.Send(api.ChatInputRename, title)
	}
}

// runMode handles /mode: with a word it sets the mode now; with none it
// asks the session for the picker.
func runMode(h Host, rest string) {
	switch word := strings.ToLower(strings.TrimSpace(rest)); word {
	case "":
		h.Ask(AskPickMode)
	case api.ChatModeAsk, api.ChatModeAuto:
		h.SetMode(word)
	default:
		h.Notice("The mode is ask or auto. Type /mode to pick one.")
	}
}

// runEffort handles /effort: a level applies from the next turn; "default"
// lets the provider choose. Which levels the model takes is the server's
// to say.
func runEffort(h Host, rest string) {
	levels := strings.Join(api.EffortLevels[:], ", ")
	switch word := strings.ToLower(strings.TrimSpace(rest)); {
	case word == "":
		_, effort := h.Model()
		if effort == "" {
			effort = "the provider's choice"
		}
		h.Notice(fmt.Sprintf("Effort is %s. Type /effort and a level: %s, or default.", effort, levels))
	case word == "default":
		h.SetEffort("")
	case slices.Contains(api.EffortLevels[:], word):
		h.SetEffort(word)
	default:
		h.Notice(fmt.Sprintf("Unknown effort %q. The levels are %s, or default.", word, levels))
	}
}

// runHelp asks the session for the help panel.
func runHelp(h Host, _ string) { h.Ask(AskHelp) }

func init() {
	for _, c := range []struct {
		cmd command.Command
		run RunFunc
	}{
		{command.Command{Name: "new", Desc: "Start a new conversation here", Group: command.GroupConversation}, runNew},
		{command.Command{Name: "resume", Desc: "Open a past conversation", Group: command.GroupConversation}, runResume},
		{command.Command{Name: "rename", Args: "[title]", Desc: "Rename this conversation", Group: command.GroupConversation}, runRename},
		{command.Command{Name: "mode", Args: "[ask|auto]", Desc: "Choose when tools wait for you", Group: command.GroupModel}, runMode},
		{command.Command{Name: "effort", Args: "[level]", Desc: "Set how hard the model thinks", Group: command.GroupModel}, runEffort},
		{command.Command{Name: "help", Desc: "Show the commands and keys", Group: command.GroupSnow}, runHelp},
	} {
		if err := Register(c.cmd, c.run); err != nil {
			panic(err)
		}
	}
}

// ResumeInput returns the input that puts the saved conversation id in
// this pane.
func (m *Model) ResumeInput(conversation string) *api.ChatInput {
	in := m.chatInput(api.ChatInputResume, "")
	in.Conversation = conversation
	return in
}

// ModeInput returns the input that sets the approval mode to mode.
func (m *Model) ModeInput(mode string) *api.ChatInput {
	in := m.chatInput(api.ChatInputMode, "")
	in.Mode = mode
	return in
}

// HelpKeys are the keys the chat's help panel lists after the commands.
var HelpKeys = []helppanel.Key{
	{Keys: "Enter", Desc: "send the message"},
	{Keys: "Shift+Enter", Desc: "add a new line"},
	{Keys: "Esc", Desc: "stop the running turn"},
	{Keys: "/", Desc: "type a command; Tab completes"},
	{Keys: "PgUp, PgDn", Desc: "scroll the conversation"},
	{Keys: "Ctrl+b :", Desc: "run a command in the session"},
	{Keys: "Ctrl+b d", Desc: "detach and go home"},
}
