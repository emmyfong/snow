package chat

import "github.com/emmyfong/snow/internal/client/command"

// runLogin asks the session for the login picker.
func runLogin(h Host, _ string) { h.Ask(AskLogin) }

// runLogout asks the session to remove a provider's saved key.
func runLogout(h Host, _ string) { h.Ask(AskLogout) }

func init() {
	if err := Register(command.Command{
		Name: "login", Desc: "Save a provider key", Group: command.GroupModel,
	}, runLogin); err != nil {
		panic(err)
	}
	if err := Register(command.Command{
		Name: "logout", Desc: "Remove a provider key", Group: command.GroupModel,
	}, runLogout); err != nil {
		panic(err)
	}
}
