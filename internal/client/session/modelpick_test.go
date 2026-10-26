package session

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// modelScreen returns a chat screen of cols by rows whose chat reports
// model.
func modelScreen(t *testing.T, cols, rows int, model string) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(cols, rows)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 2, Kind: api.PaneChat, Rect: api.Rect{W: cols, H: rows - 1}, Focused: true},
	}}}})
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: model, Mode: "ask", State: api.ChatStateIdle})
	return m, r
}

// modelList is one connected provider with two models.
func modelList() *api.ModelList {
	return &api.ModelList{Providers: []api.ProviderModels{{
		Name: "anthropic", Title: "Anthropic", Connected: true,
		Models: []api.ModelInfo{
			{ID: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5", ContextWindow: 200_000, Efforts: []string{"low", "medium", "high"}},
			{ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5", ContextWindow: 200_000},
		},
	}}}
}

// typeSlash types a command into the chat and runs it.
func typeSlash(m *Model, text string) {
	for _, r := range text {
		m.Update(press(r, 0, string(r)))
	}
	m.Update(press(tea.KeyEnter, 0, ""))
}

// lastModelInput returns the last ChatInput of kind model, or nil.
func lastModelInput(r *recorder) *api.ChatInput {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if in, ok := r.sent[i].(*api.ChatInput); ok && in.Kind == api.ChatInputModel {
			return in
		}
	}
	return nil
}

// sentListRequest reports whether the session asked the server for models.
func sentListRequest(r *recorder) bool {
	for _, msg := range r.sent {
		if _, ok := msg.(*api.ModelListRequest); ok {
			return true
		}
	}
	return false
}

// TestModelCommandOpensThePicker: /model opens the picker over the chat and
// asks the server for the provider lists.
func TestModelCommandOpensThePicker(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	typeSlash(m, "/model")
	if m.modelPick == nil {
		t.Fatal("/model did not open the model picker")
	}
	if !sentListRequest(r) {
		t.Fatal("/model did not ask for the model list")
	}
	m.Update(modelList())
	s := ansi.Strip(content(m))
	for _, want := range []string{"Model", "Anthropic", "Claude Sonnet 5.5", "effort", "ctrl+d"} {
		if !strings.Contains(s, want) {
			t.Errorf("the picker does not show %q:\n%s", want, s)
		}
	}
}

// TestModelPickSwitchesTheChat: Enter sends the chosen model to the chat's
// pane and closes the picker.
func TestModelPickSwitchesTheChat(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	typeSlash(m, "/model")
	m.Update(modelList())
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	if m.modelPick != nil {
		t.Fatal("the picker stayed open")
	}
	in := lastModelInput(r)
	if in == nil || in.Model != "anthropic/claude-haiku-5-5" || in.SaveDefault || in.Pane != 2 {
		t.Fatalf("sent %+v", in)
	}
}

// TestModelPickCtrlDSavesTheDefault: ctrl+d switches the chat and asks the
// server to save the model as the default.
func TestModelPickCtrlDSavesTheDefault(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	typeSlash(m, "/model")
	m.Update(modelList())
	m.Update(press('d', tea.ModCtrl, ""))
	in := lastModelInput(r)
	if in == nil || in.Model != "anthropic/claude-sonnet-5-5" || !in.SaveDefault {
		t.Fatalf("ctrl+d sent %+v", in)
	}
}

// TestModelPickCancelSendsNothing: Esc closes the picker without changing
// the chat.
func TestModelPickCancelSendsNothing(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	typeSlash(m, "/model")
	before := len(r.sent)
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.modelPick != nil {
		t.Fatal("Esc left the picker open")
	}
	if len(r.sent) != before || lastModelInput(r) != nil {
		t.Fatalf("Esc sent %+v", r.sent[before:])
	}
}

// TestFirstChatWithoutModelOpensThePicker: a chat with no model opens the
// picker before the first message, once. Cancel leaves it empty, and its
// next message says why it did not send.
func TestFirstChatWithoutModelOpensThePicker(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "")
	if m.modelPick == nil || !sentListRequest(r) {
		t.Fatal("a chat without a model did not open the picker")
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Mode: "ask", State: api.ChatStateIdle})
	if m.modelPick != nil {
		t.Fatal("the picker reopened after a cancel")
	}
	m.Update(press('h', 0, "h"))
	m.Update(press('i', 0, "i"))
	m.Update(press(tea.KeyEnter, 0, ""))
	if in := lastChatInput(r); in != nil {
		t.Fatalf("a chat without a model sent %+v", in)
	}
	if !strings.Contains(m.message, "Choose a model first") {
		t.Fatalf("status = %q", m.message)
	}
}

// TestCtrlBPickModel: Ctrl+b m opens the picker in a chat, and explains
// itself in a terminal pane.
func TestCtrlBPickModel(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	m.Update(ctrlB)
	m.Update(press('m', 0, "m"))
	if m.modelPick == nil || !sentListRequest(r) {
		t.Fatal("Ctrl+b m did not open the model picker")
	}

	term := attached(t, 80, 24)
	term.Update(ctrlB)
	term.Update(press('m', 0, "m"))
	if term.modelPick != nil {
		t.Fatal("the picker opened in a terminal pane")
	}
	if !strings.Contains(term.message, "chat panes") {
		t.Fatalf("status = %q", term.message)
	}
}

// TestModelPickerGolden draws the open picker over a chat.
func TestModelPickerGolden(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m, _ := modelScreen(t, size[0], size[1], "anthropic/claude-sonnet-5-5")
			typeSlash(m, "/model")
			m.Update(modelList())
			golden.RequireEqual(t, content(m))
		})
	}
}

// TestModelPickClickOutsideCancels: a left click outside the picker closes
// it without sending anything.
func TestModelPickClickOutsideCancels(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	typeSlash(m, "/model")
	before := len(r.sent)
	m.Update(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	if m.modelPick != nil {
		t.Fatal("the click outside left the picker open")
	}
	if len(r.sent) != before {
		t.Fatalf("the click sent %+v", r.sent[before:])
	}
}

// TestTopBarModelClickOpensThePicker: a click on the chat's top bar model
// asks the session for the picker.
func TestTopBarModelClickOpensThePicker(t *testing.T) {
	m, r := modelScreen(t, 100, 30, "anthropic/claude-sonnet-5-5")
	row := strings.Split(ansi.Strip(content(m)), "\n")[0]
	x := strings.Index(row, "anthropic/claude-sonnet-5-5")
	if x < 0 {
		t.Fatalf("the top bar does not show the model:\n%s", row)
	}
	m.Update(tea.MouseClickMsg(tea.Mouse{X: ansi.StringWidth(row[:x]) + 1, Y: 0, Button: tea.MouseLeft}))
	if m.modelPick == nil || !sentListRequest(r) {
		t.Fatal("the model click did not open the picker")
	}
}
