package server

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

// testSecret is the token the fake store holds. No log line, protocol
// message, or saved file may hold it.
const testSecret = "sk-test-secret-4d9f"

// helperOut names the file the helper pane writes its environment to.
const helperOut = "SNOW_HELPER_OUT"

// TestHelperPaneEnv is not a test: an agent profile pane runs the test
// binary with -test.run set to it, and it writes the variables the pane
// got to the file helperOut names.
func TestHelperPaneEnv(t *testing.T) {
	out := os.Getenv(helperOut)
	if out == "" {
		t.Skip("runs only as an agent profile's program")
	}
	var b strings.Builder
	for _, name := range []string{"TOK", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "PLAIN"} {
		fmt.Fprintf(&b, "%s=%s\n", name, os.Getenv(name))
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fakeSecrets is a SecretSource. When gate is set, a read waits for it.
type fakeSecrets struct {
	values map[string]string
	gate   chan struct{}
}

func (f fakeSecrets) APIKey(name string) (string, error) {
	if f.gate != nil {
		<-f.gate
	}
	if v, ok := f.values[name]; ok {
		return v, nil
	}
	return "", errors.New("nothing saved for " + name)
}

// hostShell is the one detected profile of these tests.
func hostShell() shell.Profile {
	if runtime.GOOS == "windows" {
		return shell.Profile{Name: "cmd", Kind: shell.KindCmd, Path: "cmd.exe"}
	}
	return shell.Profile{Name: "sh", Kind: shell.KindOther, Path: "/bin/sh"}
}

// agentProfileOptions runs the real launcher with hostShell and one agent
// profile, "Helper (free)", that runs TestHelperPaneEnv with TOK from the
// secret "omniroute".
func agentProfileOptions(t *testing.T, secrets SecretSource, log *lockedBuffer) (Options, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "env.txt")
	return Options{
		Version: "test",
		Log:     slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug})),
		detect:  func() []shell.Profile { return []shell.Profile{hostShell()} },
		Agents: []AgentProfile{{
			Name:    "Helper (free)",
			Command: []string{exe, "-test.run=^TestHelperPaneEnv$"},
			Env: []EnvVar{
				{Name: "TOK", Secret: "omniroute"},
				{Name: "PLAIN", Value: "plain value"},
				{Name: helperOut, Value: out},
			},
		}},
		Secrets: secrets,
		KeyVars: []string{"OPENAI_API_KEY"},
	}, out
}

func helperSplit(id int) *api.Command {
	return &api.Command{Op: api.OpSplit, Target: id, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgProfile: "Helper (free)"}}
}

// TestAgentProfileEnv: a pane from an agent profile gets its variables and
// the secret from the store, does not inherit key variables, and the
// secret never reaches the log, the protocol, or the saved session.
func TestAgentProfileEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-inherited")
	t.Setenv("OPENAI_API_KEY", "sk-inherited")
	var log lockedBuffer
	opts, out := agentProfileOptions(t, fakeSecrets{values: map[string]string{"omniroute": testSecret}}, &log)
	opts.SessionsDir = t.TempDir()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	var list *api.ProfileList
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	for list == nil {
		m, err := c.Next(apitest.Wait)
		if err != nil {
			t.Fatalf("no ProfileList: %v", err)
		}
		list, _ = m.(*api.ProfileList)
	}
	if want := []string{hostShell().Name, "Helper (free)"}; !slices.Equal(list.Profiles, want) {
		t.Fatalf("profiles %q, want %q", list.Profiles, want)
	}
	c.Send(helperSplit(l.Windows[0].Panes[0].ID))
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	var got []byte
	eventually(t, "the helper writes its environment", func() bool {
		b, err := os.ReadFile(out)
		got = b
		return err == nil && bytes.HasSuffix(b, []byte("PLAIN=plain value\n"))
	})
	want := "TOK=" + testSecret + "\nANTHROPIC_API_KEY=\nOPENAI_API_KEY=\nPLAIN=plain value\n"
	if string(got) != want {
		t.Fatalf("pane environment\n%s\nwant\n%s", got, want)
	}
	c.Drain(200 * time.Millisecond)
	s.Close()
	if bytes.Contains(c.Received(), []byte(testSecret)) {
		t.Fatal("the secret crossed the protocol")
	}
	if strings.Contains(log.String(), testSecret) {
		t.Fatalf("the secret is in the log:\n%s", log.String())
	}
	err := filepath.WalkDir(opts.SessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err == nil && bytes.Contains(b, []byte(testSecret)) {
			t.Errorf("the secret is saved in %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestAgentProfileSecretMissing: a pane whose secret is not saved is
// refused with an error that names the secret and how to save it. The
// store is read off the state goroutine: the server answers other
// commands while the read waits.
func TestAgentProfileSecretMissing(t *testing.T) {
	var log lockedBuffer
	gate := make(chan struct{})
	opts, out := agentProfileOptions(t, fakeSecrets{gate: gate}, &log)
	s := New(opts)
	t.Cleanup(s.Close)
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(helperSplit(l.Windows[0].Panes[0].ID))
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: "still here"}})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Name == "still here" })
	close(gate)
	deadline := time.Now().Add(apitest.Wait)
	for {
		m, err := c.Next(time.Until(deadline))
		if err != nil {
			t.Fatalf("no error for the missing secret: %v", err)
		}
		if l, ok := m.(*api.Layout); ok && len(shownPanes(l)) != 1 {
			t.Fatalf("a pane started without its secret: %+v", l)
		}
		if e, ok := m.(*api.Error); ok {
			if !strings.Contains(e.Message, "omniroute") || !strings.Contains(e.Message, "snow auth login omniroute") {
				t.Fatalf("error %q, want one that names the secret and how to save it", e.Message)
			}
			break
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the helper ran without its secret")
	}
}

// TestAgentProfileNames: a profile named like a detected one is skipped,
// a profile whose program is missing is listed and fails to start, and a
// claude program counts as an agent.
func TestAgentProfileNames(t *testing.T) {
	var log lockedBuffer
	opts := Options{
		Agents: []AgentProfile{
			{Name: hostShell().Name, Command: []string{"anything"}},
			{Name: "Missing", Command: []string{"snow-test-no-such-program"}},
			{Name: "Free", Command: []string{"/opt/bin/claude"}},
		},
	}
	set, extra := newAgentSet(opts, []shell.Profile{hostShell()}, slog.New(slog.NewTextHandler(&log, nil)))
	var names []string
	for _, p := range extra {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"Missing", "Free"}) {
		t.Fatalf("profiles %q, want [Missing Free]", names)
	}
	if !strings.Contains(log.String(), "another profile has its name") {
		t.Fatalf("no log line for the skipped profile:\n%s", log.String())
	}
	if extra[1].Kind != shell.KindClaude || !extra[1].IsAgent() {
		t.Fatalf("a claude profile is %+v, want an agent", extra[1])
	}
	l := agentLauncher(append([]shell.Profile{hostShell()}, extra...), shell.Hooks{}, set)
	if _, _, err := l.spec("Missing", ""); !errors.Is(err, shell.ErrProgramNotFound) {
		t.Fatalf("spec of a missing program: %v, want ErrProgramNotFound", err)
	}
}
