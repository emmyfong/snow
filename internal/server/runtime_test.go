package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/clauderuntime/runtimetest"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

func runtimeOptions(t *testing.T, mode string) *RuntimeOptions {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	policy := t.TempDir()
	return &RuntimeOptions{Root: t.TempDir(), Program: func(shell.Profile) (shell.Profile, error) {
		return shell.Profile{Kind: shell.KindClaude, Path: executable,
			Args: []string{"-test.run=^TestClaudeRuntimeFixture$", "--", "fixture-mode=" + mode}}, nil
	}, PolicyDir: func(string) string { return policy }}
}

func TestClaudeRuntimeFixture(_ *testing.T) {
	mode := ""
	for _, argument := range os.Args {
		if strings.HasPrefix(argument, "fixture-mode=") {
			mode = strings.TrimPrefix(argument, "fixture-mode=")
		}
	}
	if mode == "" {
		return
	}
	fixtureLog()
	for _, argument := range os.Args {
		if argument == "logout" {
			err := os.Remove(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "fixture-signed-in"))
			if err != nil && !os.IsNotExist(err) {
				os.Exit(2)
			}
			os.Exit(0)
		}
		if mode == "status-bad" && argument == "status" {
			if _, err := fmt.Fprintln(os.Stdout, `{"loggedIn":true,"authMethod":"claude.ai","configDirectory":"/not/this/connection"}`); err != nil {
				os.Exit(2)
			}
			os.Exit(0)
		}
		if mode == "login-manual" && argument == "login" {
			// A browser callback that failed: the login prints a sign-in
			// address and waits for the code the browser shows.
			if _, err := fmt.Fprint(os.Stdout, "Browser did not open. Visit this address to sign in:\r\n"+
				"https://example.invalid/oauth/authorize?state=fixture\r\n\r\nPaste code here if prompted > "); err != nil {
				os.Exit(2)
			}
			code, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil || strings.TrimSpace(code) != "fixture-code" {
				os.Exit(1)
			}
			if err := os.WriteFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "fixture-signed-in"), []byte("yes"), 0o600); err != nil {
				os.Exit(2)
			}
			os.Exit(0)
		}
		if mode == "login" && argument == "login" {
			if _, err := fmt.Fprintln(os.Stdout, "Finish sign-in: private-login-code"); err != nil {
				os.Exit(2)
			}
			if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
				os.Exit(2)
			}
			if err := os.WriteFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "fixture-signed-in"), []byte("yes"), 0o600); err != nil {
				os.Exit(2)
			}
			os.Exit(0)
		}
		if argument == "status" {
			loggedIn, method := true, "claude.ai"
			if os.Getenv("ANTHROPIC_API_KEY") != "" {
				method = "api_key"
			}
			if mode == "login" || mode == "login-manual" {
				_, err := os.Stat(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "fixture-signed-in"))
				loggedIn = err == nil
				if !loggedIn {
					method = "none"
				}
			}
			err := json.NewEncoder(os.Stdout).Encode(map[string]any{"loggedIn": loggedIn, "authMethod": method,
				"configDirectory": os.Getenv("CLAUDE_CONFIG_DIR")})
			if err != nil {
				os.Exit(2)
			}
			if loggedIn {
				os.Exit(0)
			}
			os.Exit(1)
		}
	}
	runtimetest.Conversation(mode)
}

// fixtureLogVar names a file where the fixture records each start: the
// operation, the arguments, and the variables that choose a login.
const fixtureLogVar = "SNOW_TEST_RUNTIME_LOG"

func fixtureLog() {
	path := os.Getenv(fixtureLogVar)
	if path == "" {
		return
	}
	op := "chat"
	for _, argument := range os.Args {
		if argument == "status" || argument == "login" || argument == "logout" {
			op = argument
		}
	}
	line := fmt.Sprintf("%s config=%s key=%s oauth=%s args=%s\n", op, os.Getenv("CLAUDE_CONFIG_DIR"),
		os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"), strings.Join(os.Args[1:], " "))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	if _, err := f.WriteString(line); err != nil {
		os.Exit(2)
	}
	if err := f.Close(); err != nil {
		os.Exit(2)
	}
}

func runtimeReply(t *testing.T, c *apitest.Client) *api.RuntimeConnections {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		message, _ := c.Next(100 * time.Millisecond)
		if response, ok := message.(*api.RuntimeConnections); ok {
			return response
		}
	}
	t.Fatal("runtime connection response did not arrive")
	return nil
}

func useTestRuntime(t *testing.T, c *apitest.Client, pane int) *api.ChatSnapshot {
	t.Helper()
	before := c.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Agent != "" })
	c.Send(&api.RuntimeRequest{Pane: pane, Agent: before.Agent, Action: api.RuntimeCreate, Auth: "subscription"})
	created := runtimeReply(t, c)
	if created.State == "error" || created.Selected == "" {
		t.Fatalf("create runtime connection: %+v", created)
	}
	c.Send(&api.RuntimeRequest{Pane: pane, Agent: before.Agent, Action: api.RuntimeUse, Connection: created.Selected})
	used := runtimeReply(t, c)
	if used.State == "error" {
		t.Fatalf("use runtime connection: %+v", used)
	}
	return c.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Runtime != nil && s.Agent != before.Agent })
}

func TestRuntimeChatRoundTrip(t *testing.T) {
	options, provider, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	first := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Question != nil })
	firstID := first.Question.ID
	chatInputOf(client, pane, api.ChatInputReply, func(in *api.ChatInput) { in.ID, in.Picked = firstID, []string{"A", "B"} })
	second := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Question != nil && s.Question.ID != firstID })
	secondID := second.Question.ID
	chatInputOf(client, pane, api.ChatInputReply, func(in *api.ChatInput) { in.ID, in.Text = secondID, "raw\nfree answer" })
	approval := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Pending != nil })
	approvalID := approval.Pending.ID
	chatInputOf(client, pane, api.ChatInputAnswer, func(in *api.ChatInput) { in.ID, in.Decision = approvalID, api.ChatDecisionYes })
	final := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return !s.Running && s.State == api.ChatStateIdle && s.Turn == 1 })
	if final.Runtime.Session != "backend-session" || final.Cost != 0.1 || strings.Count(client.ChatText(pane), "hello world") != 1 {
		t.Fatalf("runtime identity, accounting or streamed state lost: %+v\n%s", final, client.ChatText(pane))
	}
	if len(provider.Requests()) != 0 {
		t.Fatal("runtime chat went through the direct-API provider")
	}
}

func TestRuntimeChatFailurePreservesInput(t *testing.T) {
	options, provider, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation")
	options.Chat.Runtime.Program = func(shell.Profile) (shell.Profile, error) { return shell.Profile{}, shell.ErrProgramNotFound }
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "keep this exact request" })
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.State == api.ChatStateStopped && !s.Running })
	if !strings.Contains(client.ChatText(pane), "user: keep this exact request") || len(provider.Requests()) != 0 {
		t.Fatal("runtime failure discarded input or fell back to an API provider")
	}
}

func TestRuntimeConnectionCannotCrossEnvironment(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation")
	connection, err := clauderuntime.CreateConnection(options.Chat.Runtime.Root, "AnotherDistro", clauderuntime.Subscription)
	if err != nil {
		t.Fatal(err)
	}
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	chat := client.WaitChat(pane, func(*api.ChatSnapshot) bool { return true })
	client.Send(&api.RuntimeRequest{Pane: pane, Agent: chat.Agent, Action: api.RuntimeUse, Connection: connection.ID})
	response := runtimeReply(t, client)
	if response.State != "error" || !strings.Contains(response.Message, "another environment") {
		t.Fatalf("foreign environment selected: %+v", response)
	}
}

func TestRuntimeEventResync(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-fast")
	server := New(options)
	server.outbox = 12
	t.Cleanup(server.Close)
	watch := connect(t, server, 100, 30)
	pane := openChat(t, watch)
	useTestRuntime(t, watch, pane)
	slow := connect(t, server, 100, 30)
	slow.Send(&api.Attach{Session: "work"})
	eventually(t, "slow runtime viewer to attach", func() bool { return server.clientCount() == 2 })
	chatInputOf(watch, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	want := strings.Repeat("x", 300)
	watch.WaitChat(pane, func(s *api.ChatSnapshot) bool { return lastReply(s) == want && idle(s) })
	if server.clientCount() != 2 {
		t.Fatal("runtime event burst disconnected the slow viewer")
	}
	resynced := slow.WaitChat(pane, func(s *api.ChatSnapshot) bool { return lastReply(s) == want && idle(s) })
	if resynced.Runtime == nil || resynced.Runtime.Session != "backend-session" {
		t.Fatal("snapshot resynchronization lost runtime identity")
	}
}

func TestRuntimeResumeKeepsSessionWithoutRestartingWork(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-fast")
	program := options.Chat.Runtime.Program
	var starts atomic.Int32
	options.Chat.Runtime.Program = func(host shell.Profile) (shell.Profile, error) {
		starts.Add(1)
		return program(host)
	}
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	first := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == 1 })
	conversation, generation := first.Conversation, first.Agent
	chatInputOf(client, pane, api.ChatInputNew, nil)
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Conversation != conversation && s.Runtime != nil })
	chatInputOf(client, pane, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = conversation })
	resumed := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Conversation == conversation })
	if resumed.Agent == generation || resumed.Runtime.Session != "backend-session" || resumed.Running || starts.Load() != 1 {
		t.Fatalf("resume restarted work, lost identity or reused permissions: %+v; starts %d", resumed, starts.Load())
	}
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "explicit continuation" })
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == 2 })
	if starts.Load() != 2 {
		t.Fatal("explicit continuation did not start exactly one resumed runtime")
	}
}
