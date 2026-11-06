package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// lastRuntimeRequest returns the last connection request sent, or nil.
func lastRuntimeRequest(r *recorder) *api.RuntimeRequest {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if request, ok := r.sent[i].(*api.RuntimeRequest); ok {
			return request
		}
	}
	return nil
}

// TestAddedConnectionIsUsed: after a new Claude plan connection signs in,
// the chat switches to it and the picker closes. A sign-in that fails adds
// nothing, so nothing is used (#290).
func TestAddedConnectionIsUsed(t *testing.T) {
	for _, tt := range []struct {
		name     string
		selected string
		want     string
	}{{"signed in", "n3wc0nn3ct", api.RuntimeUse}, {"failed", "", api.RuntimeCreate}} {
		t.Run(tt.name, func(t *testing.T) {
			m, r := runtimeModelScreen(t, 100, 30)
			m.openRuntimePicker(2)
			reply := func(response api.RuntimeConnections) {
				request := lastRuntimeRequest(r)
				response.RequestID, response.Pane, response.Agent = request.RequestID, 2, request.Agent
				m.Update(&response)
			}
			reply(api.RuntimeConnections{Connections: []api.RuntimeConnection{}})
			m.Update(press(tea.KeyEnter, 0, ""))
			if got := lastRuntimeRequest(r); got.Action != api.RuntimeCreate || got.Auth != "subscription" {
				t.Fatalf("Enter on the first row sent %+v, want a Claude plan create", got)
			}
			reply(api.RuntimeConnections{State: "signing-in", LoginPane: 9, Selected: "n3wc0nn3ct"})
			reply(api.RuntimeConnections{State: "updated", Selected: tt.selected, Message: "done"})
			if got := lastRuntimeRequest(r); got.Action != tt.want || got.Connection != tt.selected {
				t.Fatalf("after the sign-in sent %+v, want %s %q", got, tt.want, tt.selected)
			}
		})
	}
}

// TestAddedKeyConnectionClosesPicker: an API key connection that was added
// with its key is used, and the picker closes once the chat uses it.
func TestAddedKeyConnectionClosesPicker(t *testing.T) {
	m, r := runtimeModelScreen(t, 100, 30)
	m.openRuntimePicker(2)
	reply := func(response api.RuntimeConnections) {
		request := lastRuntimeRequest(r)
		response.RequestID, response.Pane, response.Agent = request.RequestID, 2, request.Agent
		m.Update(&response)
	}
	reply(api.RuntimeConnections{Connections: []api.RuntimeConnection{}})
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	m.Update(tea.PasteMsg{Content: "sk-ant-new"})
	m.Update(press(tea.KeyEnter, 0, ""))
	if got := lastRuntimeRequest(r); got.Action != api.RuntimeCreate || got.Key != "sk-ant-new" {
		t.Fatalf("submitting the key sent %v, want a create with the key", got)
	}
	added := api.RuntimeConnection{ID: "k3yc0nn3ct", Name: "Claude Code", Auth: "api-key", Environment: "This machine", Status: "key-saved"}
	reply(api.RuntimeConnections{State: "updated", Selected: added.ID, Connections: []api.RuntimeConnection{added}})
	if got := lastRuntimeRequest(r); got.Action != api.RuntimeUse || got.Connection != added.ID {
		t.Fatalf("after adding sent %v, want use %s", got, added.ID)
	}
	reply(api.RuntimeConnections{State: "updated", Selected: added.ID, Connections: []api.RuntimeConnection{added}})
	if m.runtimePick != nil {
		t.Fatal("the picker stayed open after the chat used the new connection")
	}
}
