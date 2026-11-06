package server

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
	"github.com/zalando/go-keyring"
)

// runtimeChat is a chat pane the acceptance tests send connection
// requests from.
type runtimeChat struct {
	t     *testing.T
	c     *apitest.Client
	pane  int
	agent string
}

func newRuntimeChat(t *testing.T, server *Server) *runtimeChat {
	t.Helper()
	c := connect(t, server, 100, 30)
	pane := openChat(t, c)
	chat := c.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Agent != "" })
	return &runtimeChat{t: t, c: c, pane: pane, agent: chat.Agent}
}

// do sends one connection request and returns its answer.
func (r *runtimeChat) do(action, connection, auth, key string) *api.RuntimeConnections {
	r.t.Helper()
	id := fmt.Sprintf("%s-%d", action, time.Now().UnixNano())
	r.c.Send(&api.RuntimeRequest{RequestID: id, Pane: r.pane, Agent: r.agent, Action: action, Connection: connection, Auth: auth, Key: key})
	for {
		response := runtimeReply(r.t, r.c)
		if response.RequestID == id && response.State != "signing-in" {
			if response.Agent != "" {
				r.agent = response.Agent
			}
			return response
		}
	}
}

func (r *runtimeChat) create(auth string) string {
	r.t.Helper()
	response := r.do(api.RuntimeCreate, "", auth, "")
	if response.State == "error" || response.Selected == "" {
		r.t.Fatalf("create %s connection: %+v", auth, response)
	}
	return response.Selected
}

func (r *runtimeChat) status(id string) api.RuntimeConnection {
	r.t.Helper()
	response := r.do(api.RuntimeStatus, id, "", "")
	for _, connection := range response.Connections {
		if connection.ID == id {
			return connection
		}
	}
	r.t.Fatalf("status of %s missing: %+v", id, response)
	return api.RuntimeConnection{}
}

func (r *runtimeChat) use(id string) {
	r.t.Helper()
	if response := r.do(api.RuntimeUse, id, "", ""); response.State == "error" {
		r.t.Fatalf("use %s: %+v", id, response)
	}
	r.c.WaitChat(r.pane, func(s *api.ChatSnapshot) bool { return s.Runtime != nil && s.Runtime.Connection == id })
}

// signIn runs the fixture's official login for id and finishes it the way
// a user would, in the sign-in pane.
func (r *runtimeChat) signIn(id string) *api.RuntimeConnections {
	r.t.Helper()
	r.c.Send(&api.RuntimeRequest{RequestID: "login-" + id, Pane: r.pane, Agent: r.agent, Action: api.RuntimeLogin, Connection: id})
	opened := runtimeReply(r.t, r.c)
	if opened.State != "signing-in" || opened.LoginPane == 0 {
		r.t.Fatalf("sign-in for %s did not open: %+v", id, opened)
	}
	r.c.WaitScreen("private-login-code")
	r.c.Send(&api.Input{Pane: opened.LoginPane, Paste: "finish\r"})
	for {
		if response := runtimeReply(r.t, r.c); response.RequestID == "login-"+id {
			return response
		}
	}
}

func readFixtureLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func useFixtureLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.log")
	t.Setenv(fixtureLogVar, path)
	return path
}

func signedInMarker(root, id string) string {
	return filepath.Join(root, "connections", id, "claude", "fixture-signed-in")
}

// TestConnectionLeavesStandaloneLogin: a signed-in standalone Claude, found
// through the host's CLAUDE_CONFIG_DIR, is not claimed by a new connection,
// and sign-in and sign-out of the connection never touch its folder.
func TestConnectionLeavesStandaloneLogin(t *testing.T) {
	standalone := t.TempDir()
	for name, body := range map[string]string{"fixture-signed-in": "yes", "settings.json": `{"model":"mine"}`} {
		if err := os.WriteFile(filepath.Join(standalone, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", standalone)
	log := useFixtureLog(t)
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	id := r.create("subscription")
	if got := r.status(id); got.Status != "signed-out" {
		t.Fatalf("new connection claimed the standalone login: %+v", got)
	}
	if got := r.signIn(id); got.Connections[0].Status != "signed-in" {
		t.Fatalf("sign-in did not finish: %+v", got)
	}
	if got := r.do(api.RuntimeLogout, id, "", ""); got.State == "error" {
		t.Fatalf("sign-out: %+v", got)
	}
	if got := r.status(id); got.Status != "signed-out" {
		t.Fatalf("sign-out left the connection signed in: %+v", got)
	}
	entries, err := os.ReadDir(standalone)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("standalone folder changed: %v", entries)
	}
	if data, err := os.ReadFile(filepath.Join(standalone, "settings.json")); err != nil || string(data) != `{"model":"mine"}` {
		t.Fatalf("standalone settings changed: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(standalone, "fixture-signed-in")); err != nil {
		t.Fatal("standalone login was signed out")
	}
	for _, line := range readFixtureLog(t, log) {
		if strings.Contains(line, "config="+standalone+" ") {
			t.Fatalf("a connection ran with the standalone folder: %s", line)
		}
	}
}

// TestTwoConnectionsStayApart: signing one connection in or out leaves the
// other as it was.
func TestTwoConnectionsStayApart(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	a, b := r.create("subscription"), r.create("subscription")
	r.signIn(a)
	if got := r.status(b); got.Status != "signed-out" {
		t.Fatalf("signing in A signed in B: %+v", got)
	}
	r.signIn(b)
	r.do(api.RuntimeLogout, a, "", "")
	if got := r.status(b); got.Status != "signed-in" {
		t.Fatalf("signing out A signed out B: %+v", got)
	}
	if got := r.status(a); got.Status != "signed-out" {
		t.Fatalf("A is still signed in: %+v", got)
	}
}

// TestHostEnvDoesNotChooseRoute: a host API key and OAuth token neither sign
// in a subscription connection nor serve an API-key connection that has no
// saved key. Snow names them as ignored, and no runtime receives them.
func TestHostEnvDoesNotChooseRoute(t *testing.T) {
	keyring.MockInit()
	t.Setenv("ANTHROPIC_API_KEY", "host-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "host-token")
	log := useFixtureLog(t)
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	options.Chat.Runtime.Keys = credential.New(t.TempDir(), nil)
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	sub := r.create("subscription")
	if got := r.status(sub); got.Status != "signed-out" || got.Auth != "subscription" {
		t.Fatalf("host variables changed the subscription connection: %+v", got)
	}
	key := r.create("api-key")
	if got := r.status(key); got.Status != "signed-out" {
		t.Fatalf("host key served an API-key connection: %+v", got)
	}
	response := r.do(api.RuntimeList, "", "", "")
	for _, name := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if !slices.Contains(response.Ignored, name) {
			t.Errorf("ignored host variable %s not shown: %v", name, response.Ignored)
		}
	}
	for _, line := range readFixtureLog(t, log) {
		if strings.Contains(line, "host-key") || strings.Contains(line, "host-token") {
			t.Fatalf("a runtime received a host credential: %s", line)
		}
	}
}

// TestAPIKeyLookupIgnoresHostEnv: a connection's runtime gets only the key
// saved for that connection: not the host's, not the Anthropic provider's,
// and not another connection's. Saving it changes no other key.
func TestAPIKeyLookupIgnoresHostEnv(t *testing.T) {
	keyring.MockInit()
	t.Setenv("ANTHROPIC_API_KEY", "host-key")
	log := useFixtureLog(t)
	keys := credential.New(t.TempDir(), nil)
	providerKey, err := credential.NewKey("provider-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keys.Set("anthropic", providerKey); err != nil {
		t.Fatal(err)
	}
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-fast")
	options.Chat.Runtime.Keys = keys
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	one, two := r.create("api-key"), r.create("api-key")
	if got := r.do(api.RuntimeSaveKey, one, "", "connection-one-key"); got.State == "error" {
		t.Fatalf("save key: %+v", got)
	}
	if got := r.status(one); got.Status != "key-saved" {
		t.Fatalf("saved key not found: %+v", got)
	}
	if got := r.status(two); got.Status != "signed-out" {
		t.Fatalf("another connection's key was used: %+v", got)
	}
	if k, _, err := keys.GetStored("anthropic"); err != nil || k.Reveal() != "provider-key" {
		t.Fatalf("provider key changed: %v", err)
	}
	r.use(one)
	chatInputOf(r.c, r.pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "hello" })
	r.c.WaitChat(r.pane, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == 1 })
	lines := readFixtureLog(t, log)
	if len(lines) == 0 {
		t.Fatal("the runtime did not start")
	}
	for _, line := range lines {
		if !strings.Contains(line, " key=connection-one-key ") {
			t.Fatalf("runtime did not get exactly the connection's key: %s", line)
		}
	}
}

// TestLoginCancelRefreshesState: a cancel that races the login's own end
// reports the state the connection really has, and never signs out.
func TestLoginCancelRefreshesState(t *testing.T) {
	log := useFixtureLog(t)
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	id := r.create("subscription")
	r.c.Send(&api.RuntimeRequest{RequestID: "login", Pane: r.pane, Agent: r.agent, Action: api.RuntimeLogin, Connection: id})
	opened := runtimeReply(t, r.c)
	if opened.LoginPane == 0 {
		t.Fatalf("sign-in did not open: %+v", opened)
	}
	r.c.WaitScreen("private-login-code")
	r.c.Send(&api.Input{Pane: opened.LoginPane, Paste: "finish\r"})
	r.c.Send(&api.RuntimeRequest{RequestID: "cancel", Pane: r.pane, Agent: r.agent, Action: api.RuntimeCancelLogin, Connection: id})
	var finished *api.RuntimeConnections
	for finished == nil {
		if response := runtimeReply(t, r.c); response.RequestID == "login" {
			finished = response
		}
	}
	want := "signed-out"
	if _, err := os.Stat(signedInMarker(options.Chat.Runtime.Root, id)); err == nil {
		want = "signed-in"
	}
	if finished.State == "error" || finished.Connections[0].Status != want {
		t.Fatalf("after cancel: %+v, want real state %s", finished, want)
	}
	for _, line := range readFixtureLog(t, log) {
		if strings.HasPrefix(line, "logout ") {
			t.Fatal("cancel signed out")
		}
	}
}

// TestStatusUnknownIsNotSignedOut: when Snow cannot learn the state, the
// connection shows unknown and why, never signed out.
func TestStatusUnknownIsNotSignedOut(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		prepare func(t *testing.T, o *RuntimeOptions)
	}{
		{"status for another folder", "status-bad", nil},
		{"broken policy file", "login", func(t *testing.T, o *RuntimeOptions) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "managed-settings.json"), []byte("{broken"), 0o600); err != nil {
				t.Fatal(err)
			}
			o.PolicyDir = func(string) string { return dir }
		}},
		{"claude not installed", "login", func(_ *testing.T, o *RuntimeOptions) {
			o.Program = func(shell.Profile) (shell.Profile, error) { return shell.Profile{}, shell.ErrProgramNotFound }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, _, _ := chatOptions(t)
			options.Chat.Runtime = runtimeOptions(t, tt.mode)
			if tt.prepare != nil {
				tt.prepare(t, options.Chat.Runtime)
			}
			connection, err := clauderuntime.CreateConnection(options.Chat.Runtime.Root, "", clauderuntime.Subscription)
			if err != nil {
				t.Fatal(err)
			}
			server := New(options)
			t.Cleanup(server.Close)
			r := newRuntimeChat(t, server)
			if got := r.status(connection.ID); got.Status != "unknown" || got.Reason == "" {
				t.Fatalf("status = %+v, want unknown with a reason", got)
			}
		})
	}
}

// TestConnectionSecretsNotExposed: a saved key reaches the runtime only
// through its environment. It is absent from the server log, the
// connection files, the conversation files, the arguments, and the
// formatted request.
func TestConnectionSecretsNotExposed(t *testing.T) {
	const secret = "sk-ant-secret-0123456789"
	keyring.MockInit()
	log := useFixtureLog(t)
	var logged lockedBuffer
	conversations := filepath.Join(t.TempDir(), "conversations")
	options, _, _ := chatOptionsIn(t, conversations)
	options.Log = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))
	options.Chat.Runtime = runtimeOptions(t, "conversation-fast")
	options.Chat.Runtime.Keys = credential.New(t.TempDir(), nil)
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)
	id := r.create("api-key")
	request := &api.RuntimeRequest{Pane: r.pane, Agent: r.agent, Action: api.RuntimeSaveKey, Connection: id, Key: secret}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", request, request, *request), secret) {
		t.Fatal("formatted request shows the key")
	}
	if got := r.do(api.RuntimeSaveKey, id, "", secret); got.State == "error" || strings.Contains(got.Message, secret) {
		t.Fatalf("save key: %+v", got)
	}
	r.use(id)
	chatInputOf(r.c, r.pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "hello" })
	r.c.WaitChat(r.pane, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == 1 })
	server.Close()
	for _, dir := range []string{options.Chat.Runtime.Root, conversations} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(data), secret) {
				t.Errorf("%s holds the key", path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(logged.String(), secret) {
		t.Error("server log holds the key")
	}
	lines := readFixtureLog(t, log)
	if len(lines) == 0 {
		t.Fatal("the runtime did not start")
	}
	for _, line := range lines {
		_, args, _ := strings.Cut(line, " args=")
		if strings.Contains(args, secret) {
			t.Fatalf("arguments hold the key: %s", line)
		}
		if !strings.Contains(line, " key="+secret+" ") {
			t.Fatalf("runtime environment lacks the key: %s", line)
		}
	}
}

// TestManagedPolicyConflictShown: a managed rule refuses a route clearly,
// creates or changes nothing, and opens no sign-in.
func TestManagedPolicyConflictShown(t *testing.T) {
	policy := t.TempDir()
	writeRule := func(method string) {
		body := fmt.Sprintf(`{"forceLoginMethod":%q}`, method)
		if err := os.WriteFile(filepath.Join(policy, "managed-settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	options.Chat.Runtime.PolicyDir = func(string) string { return policy }
	existing, err := clauderuntime.CreateConnection(options.Chat.Runtime.Root, "", clauderuntime.Subscription)
	if err != nil {
		t.Fatal(err)
	}
	server := New(options)
	t.Cleanup(server.Close)
	r := newRuntimeChat(t, server)

	writeRule("claudeai")
	response := r.do(api.RuntimeCreate, "", "api-key", "")
	if response.State != "error" || !strings.Contains(response.Message, "requires Claude subscription sign-in") {
		t.Fatalf("API-key route not refused clearly: %+v", response)
	}
	if len(response.Connections) != 1 {
		t.Fatalf("a refused route created a connection: %+v", response.Connections)
	}

	writeRule("console")
	if got := r.status(existing.ID); got.Status != "blocked" || !strings.Contains(got.Reason, "Claude Console") {
		t.Fatalf("policy conflict not shown: %+v", got)
	}
	if got := r.do(api.RuntimeUse, existing.ID, "", ""); got.State != "error" || !strings.Contains(got.Message, "Claude Console") {
		t.Fatalf("blocked connection used: %+v", got)
	}
	if got := r.do(api.RuntimeLogin, existing.ID, "", ""); got.State != "error" || got.LoginPane != 0 {
		t.Fatalf("blocked connection opened a sign-in: %+v", got)
	}
}

// TestRestartStartsNoLoginOrTurn: after a restart, a restored chat on a
// signed-in connection starts no Claude process at all until the user acts:
// no sign-in, no status check, and no turn.
func TestRestartStartsNoLoginOrTurn(t *testing.T) {
	log := useFixtureLog(t)
	d := newChatDirs(t)
	runtime := runtimeOptions(t, "conversation-fast")
	start := func() *Server {
		opts, _, _ := chatOptionsIn(t, d.convs)
		opts.SessionsDir = d.sessions
		opts.saveDelay = 20 * time.Millisecond
		opts.busyEvery = time.Hour
		opts.Chat.Runtime = runtime
		s := New(opts)
		t.Cleanup(s.Close)
		return s
	}
	first := start()
	r := newRuntimeChat(t, first)
	id := r.create("subscription")
	r.use(id)
	chatInputOf(r.c, r.pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "hello" })
	done := r.c.WaitChat(r.pane, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == 1 })
	eventually(t, "the runtime chat saved", func() bool { return chatPaneOf(t, d, "work").Conversation == done.Conversation })
	stopBySignal(first)
	before := len(readFixtureLog(t, log))

	second := start()
	c := connect(t, second, 100, 30)
	c.Send(&api.Attach{Session: "work"})
	l := nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 })
	restored := c.WaitChat(chatIn(l), func(s *api.ChatSnapshot) bool { return s.Conversation == done.Conversation })
	if restored.Runtime == nil || restored.Runtime.Connection != id || restored.Running {
		t.Fatalf("restored chat lost its connection or runs: %+v", restored)
	}
	time.Sleep(300 * time.Millisecond)
	if after := readFixtureLog(t, log); len(after) != before {
		t.Fatalf("restart started Claude: %v", after[before:])
	}
}
