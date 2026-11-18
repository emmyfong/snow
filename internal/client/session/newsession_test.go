package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/newsession"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func TestOpenNewWindowDialog(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	if cmd := m.openNewWindow(); cmd != nil || m.over == nil {
		t.Fatalf("openNewWindow: overlay %v, command %v; want dialog", m.over != nil, cmd)
	}
	readiness := r.sent[0].(*api.RuntimeRequest)
	if readiness.Pane != 0 || readiness.Profile != "Ubuntu" || readiness.Folder != "/src" {
		t.Fatalf("window readiness request %+v, want focused Ubuntu profile and folder /src", readiness)
	}
	pick, ok := m.over.popup.(*newsession.Model)
	if !ok {
		t.Fatalf("overlay popup is %T, want new-session dialog", m.over.popup)
	}
	if got := pick.Choice().Folder; got != "/src" {
		t.Fatalf("dialog folder = %q, want focused pane folder /src", got)
	}
	for range 2 {
		m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	cmd := lastCommand(t, r)
	if m.over != nil || cmd.Op != api.OpNewWindow || cmd.Args[api.ArgFolder] != "/src" {
		t.Fatalf("accept: overlay %v, command %+v; want new window in /src", m.over != nil, cmd)
	}
}

func TestNewWindowLayout(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	for range 2 {
		m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if got := lastCommand(t, r); got.Op != api.OpNewWindow {
		t.Fatalf("accepted dialog sent %+v, want new window", got)
	}
	added := &api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Panes: []api.PaneInfo{{ID: 1}}},
		{Index: 2, Panes: []api.PaneInfo{{ID: 99}, {ID: 100}}},
	}}
	m.Update(added)
	got := lastCommand(t, r)
	if got.Op != api.OpLayout || got.Target != 99 || got.Args[api.ArgName] != "ide" {
		t.Fatalf("new layout command %+v, want layout ide for first pane 99", got)
	}
	count := len(r.sent)
	m.Update(added)
	if len(r.sent) != count {
		t.Fatalf("second layout sent more commands: %#v", r.sent[count:])
	}
}

func TestFailedNewWindowDoesNotReuseChoices(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	m.Update(&api.RuntimeConnections{Pane: 0, Agent: api.AgentClaude, RequestID: m.windowRuntimeRequest})
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if m.newPreset == "" || !m.newWindowPending {
		t.Fatalf("first choice did not stage a layout: preset=%q pending=%v", m.newPreset, m.newWindowPending)
	}
	m.Update(&api.Error{Code: api.CodeCommandFailed, Message: "window creation failed"})
	if m.newPreset != "" || m.pendingWindowRuntime != "" || m.newWindowPending {
		t.Fatalf("failed window retained choices: preset=%q runtime=%q pending=%v", m.newPreset, m.pendingWindowRuntime, m.newWindowPending)
	}
	m.openNewWindow()
	m.Update(&api.RuntimeConnections{Pane: 0, Agent: api.AgentClaude, RequestID: m.windowRuntimeRequest})
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	added := &api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Panes: []api.PaneInfo{{ID: 1}}},
		{Index: 2, Panes: []api.PaneInfo{{ID: 99, Kind: api.PaneTerminal}}},
	}}
	before := len(r.sent)
	m.Update(added)
	for _, message := range r.sent[before:] {
		if command, ok := message.(*api.Command); ok && command.Op == api.OpLayout {
			t.Fatalf("retry with No layout reused a prior preset: %+v", command)
		}
	}
}

func TestNewSessionWindowEscape(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.over != nil || len(r.sent) != 1 {
		t.Fatalf("Esc left overlay %v and sent %v; want closed without a command", m.over != nil, r.sent)
	}
	if request, ok := r.sent[0].(*api.RuntimeRequest); !ok || request.Pane != 0 || request.Agent != api.AgentClaude {
		t.Fatalf("background readiness request = %#v, want pane-zero Claude list", r.sent[0])
	}
}

func TestNewWindowClaudeUsesReadyConnection(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.openNewWindow()
	m.Update(&api.RuntimeConnections{Pane: 0, Agent: api.AgentClaude, RequestID: m.windowRuntimeRequest,
		Connections: []api.RuntimeConnection{{ID: "ready", Status: api.RuntimeStatusSignedIn, Ready: true}}})
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	command := lastCommand(t, r)
	if command.Op != api.OpNewWindow || command.Args[api.ArgKind] != api.PaneChat {
		t.Fatalf("Claude window command %+v", command)
	}
	added := &api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Panes: []api.PaneInfo{{ID: 1, Kind: api.PaneChat}}},
		{Index: 2, Panes: []api.PaneInfo{{ID: 99, Kind: api.PaneChat}}},
	}}
	m.Update(added)
	m.ownMsg(&api.ChatSnapshot{Pane: 99, Agent: "chat-generation", State: api.ChatStateIdle})
	var request *api.RuntimeRequest
	for _, message := range r.sent {
		if sent, ok := message.(*api.RuntimeRequest); ok && sent.Pane == 99 {
			request = sent
		}
	}
	if request == nil || request.Action != api.RuntimeUse || request.Connection != "ready" {
		t.Fatalf("window chat binding request %+v, want ready connection", request)
	}
}

func TestReadyClaudeKeepsMinimumHeightWindowDialogVisible(t *testing.T) {
	m, _ := pickerScreen(t, 80, 17)
	m.openNewWindow()
	m.Update(&api.RuntimeConnections{Pane: 0, Agent: api.AgentClaude, RequestID: m.windowRuntimeRequest,
		Connections: []api.RuntimeConnection{{ID: "ready", Status: api.RuntimeStatusSignedIn, Ready: true}}})
	if m.over == nil {
		t.Fatal("ready Claude reply closed the window dialog at minimum height")
	}
	dialog := m.over.popup.(*newsession.Model)
	box, _, _ := dialog.View(m.theme, 80, m.windowRows())
	if box == "" {
		t.Fatal("ready Claude reply made the window dialog invisible at minimum height")
	}
}

func TestNewSessionClaudeOnePane(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingRuntime("ready-connection")
	m.Update(&api.Layout{Session: "new", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 99, Kind: api.PaneChat}}}}})
	m.Update(&api.ChatSnapshot{Pane: 99, Agent: "chat-generation", State: api.ChatStateIdle})
	var request *api.RuntimeRequest
	for _, message := range r.sent {
		if sent, ok := message.(*api.RuntimeRequest); ok {
			request = sent
		}
		if command, ok := message.(*api.Command); ok && (command.Op == api.OpSplit || command.Op == api.OpClose) {
			t.Fatalf("runtime startup changed panes with %+v", command)
		}
	}
	if request == nil || request.Pane != 99 || request.Agent != "chat-generation" || request.Action != api.RuntimeUse || request.Connection != "ready-connection" {
		t.Fatalf("runtime request %+v, want to bind the first chat to the selected connection", request)
	}
	if request.RequestID == "" || !m.runtimeBindFailed && m.runtimeBinding != "ready-connection" {
		t.Fatalf("binding state request=%+v, binding=%q failed=%v", request, m.runtimeBinding, m.runtimeBindFailed)
	}
	before := len(r.sent)
	if _, ok := m.own[99].(runtimeChooser); !ok {
		t.Fatalf("startup pane type %T is not runtime chooser", m.own[99])
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	if len(r.sent) != before {
		t.Fatalf("chat input raced runtime binding: %#v", r.sent[before:])
	}
	m.Update(&api.RuntimeConnections{Pane: 99, Agent: "chat-generation", RequestID: request.RequestID, Selected: "ready-connection", State: "updated"})
	if m.runtimeBinding != "" || m.runtimeBindFailed {
		t.Fatalf("successful bind left state binding=%q failed=%v after=%q pane=%d request=%q response=%q", m.runtimeBinding, m.runtimeBindFailed, m.runtimeAfter, m.runtimePane, m.runtimeRequestID, request.RequestID)
	}
}

func TestNewSessionClaudeBindingFailureBlocksChat(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingRuntime("missing-connection")
	m.Update(&api.Layout{Session: "new", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal}, {ID: 99, Kind: api.PaneChat, Focused: true},
	}}}})
	m.ownMsg(&api.ChatSnapshot{Pane: 99, Agent: "chat-generation", State: api.ChatStateIdle})
	var request *api.RuntimeRequest
	for _, message := range r.sent {
		if sent, ok := message.(*api.RuntimeRequest); ok {
			request = sent
		}
	}
	if request == nil {
		t.Fatal("startup did not request the selected runtime")
	}
	m.Update(&api.RuntimeConnections{Pane: 99, Agent: "chat-generation", RequestID: request.RequestID, State: "error", Message: "connection disappeared"})
	if !m.runtimeBindFailed || m.message != "connection disappeared" {
		t.Fatalf("failed binding state=%v message=%q after=%q pane=%d request=%q agent=%q", m.runtimeBindFailed, m.message, m.runtimeAfter, m.runtimePane, m.runtimeRequestID, request.Agent)
	}
	before := len(r.sent)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	if len(r.sent) != before {
		t.Fatalf("chat input sent after failed binding: %#v", r.sent[before:])
	}
	m.Update(&api.Layout{Session: "new", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Focused: true}, {ID: 99, Kind: api.PaneChat},
	}}}})
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if len(r.sent) != before+1 {
		t.Fatalf("failed chat disabled input in unrelated terminal: %#v", r.sent[before:])
	}
	input, ok := r.sent[before].(*api.Input)
	if !ok || input.Pane != 1 || input.Key == nil || input.Key.Text != "y" {
		t.Fatalf("unrelated pane input %#v, want key sent to terminal 1", r.sent[before])
	}
}

func TestFailedClaudeBindingCanReachRuntimePickerThroughLogin(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	m.runtimeBindFailed, m.runtimePane = true, 2
	typeSlash(m, "/login")
	if m.authPick == nil {
		t.Fatal("failed chat blocked /login recovery")
	}
	m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "claude-code", Title: "Claude Code", Runtime: "claude-code"}}})
	m.Update(press(tea.KeyEnter, 0, ""))
	if m.runtimePick == nil {
		t.Fatal("Claude Code login choice did not open the runtime picker")
	}
}

func TestNewSessionPlainShellReplacesInitialChat(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingShell()
	m.SetPendingLayout(api.LayoutPresets[0].Name)
	initial := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 7, Kind: api.PaneChat}}}}}
	m.Update(initial)
	split := lastCommand(t, r)
	if split.Op != api.OpSplit || split.Target != 7 || split.Args[api.ArgKind] != api.PaneTerminal {
		t.Fatalf("initial shell conversion command %+v", split)
	}
	withTerminal := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 7, Kind: api.PaneChat}, {ID: 8, Kind: api.PaneTerminal},
	}}}}
	m.Update(withTerminal)
	closeCmd := lastCommand(t, r)
	if closeCmd.Op != api.OpClose || closeCmd.Target != 7 {
		t.Fatalf("temporary chat close command %+v", closeCmd)
	}
	final := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 8, Kind: api.PaneTerminal}}}}}
	m.Update(final)
	layout := lastCommand(t, r)
	if layout.Op != api.OpLayout || layout.Target != 8 || layout.Args[api.ArgName] != api.LayoutPresets[0].Name || m.pendingShell || m.shellStart != 0 {
		t.Fatalf("final shell transition command %+v, pending=%v phase=%d", layout, m.pendingShell, m.shellStart)
	}
}

func TestNewSessionPlainShellHandlesRestoredPaneOrder(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingShell()
	initial := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 5, Kind: api.PaneTerminal}, {ID: 7, Kind: api.PaneChat, Focused: true}, {ID: 8, Kind: api.PaneChat},
	}}}}
	m.Update(initial)
	if split := lastCommand(t, r); split.Op != api.OpSplit || split.Target != 7 {
		t.Fatalf("restored layout split command %+v, want focused chat 7", split)
	}
	withSplit := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 5, Kind: api.PaneTerminal}, {ID: 7, Kind: api.PaneChat}, {ID: 8, Kind: api.PaneChat}, {ID: 9, Kind: api.PaneTerminal},
	}}}}
	m.Update(withSplit)
	if closeCmd := lastCommand(t, r); closeCmd.Op != api.OpClose || closeCmd.Target != 7 {
		t.Fatalf("restored chat close command %+v", closeCmd)
	}
	firstClosed := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 5, Kind: api.PaneTerminal}, {ID: 8, Kind: api.PaneChat}, {ID: 9, Kind: api.PaneTerminal},
	}}}}
	m.Update(firstClosed)
	if closeCmd := lastCommand(t, r); closeCmd.Op != api.OpClose || closeCmd.Target != 8 {
		t.Fatalf("additional restored chat close command %+v", closeCmd)
	}
	final := &api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{
		{ID: 5, Kind: api.PaneTerminal}, {ID: 9, Kind: api.PaneTerminal},
	}}}}
	m.Update(final)
	if layout := lastCommand(t, r); layout.Op != api.OpLayout || layout.Target != 9 || m.pendingShell || m.shellStart != 0 {
		t.Fatalf("restored shell completion command %+v, pending=%v phase=%d", layout, m.pendingShell, m.shellStart)
	}
}

func TestPlainShellStartupBuffersEarlyKeysAndPaste(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.SetPendingShell()
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	m.Update(tea.PasteMsg{Content: "first command"})
	if len(m.early) != 2 || len(r.sent) != 0 {
		t.Fatalf("startup buffered %d inputs and sent %v, want two buffered inputs", len(m.early), r.sent)
	}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 44, Kind: api.PaneTerminal, Focused: true}}}}})
	if m.pendingShell || len(m.early) != 0 || len(r.sent) != 2 {
		t.Fatalf("shell start pending=%v early=%v sent=%v", m.pendingShell, m.early, r.sent)
	}
	key, ok := r.sent[0].(*api.Input)
	paste, pasteOK := r.sent[1].(*api.Input)
	if !ok || key.Pane != 44 || key.Key == nil || key.Key.Text != "x" || !pasteOK || paste.Pane != 44 || paste.Paste != "first command" {
		t.Fatalf("flushed inputs %#v, want key and paste sent to terminal 44", r.sent)
	}
}

func TestNewSessionFromWindowBarGolden80x24(t *testing.T)  { newSessionFromWindowBarGolden(t, 80, 24) }
func TestNewSessionFromWindowBarGolden120x40(t *testing.T) { newSessionFromWindowBarGolden(t, 120, 40) }

func newSessionFromWindowBarGolden(t *testing.T, cols, rows int) {
	m, _ := pickerScreen(t, cols, rows)
	_, spans := ui.WindowBarRow(m.theme, m.windowBarOptions())
	for _, span := range spans {
		if span.Kind == ui.TopBarNewWindow {
			m.Update(click(span.X, 0))
			if m.over == nil {
				t.Fatal("the + control did not open the dialog")
			}
			golden.RequireEqual(t, m.View().Content)
			return
		}
	}
	t.Fatal("window bar has no + control")
}
