package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// helper returns a Helper that starts this test binary and appends the
// notice it is given to file.
func helper(t *testing.T, file string) func(context.Context, Notice) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, n Notice) *exec.Cmd {
		return exec.CommandContext(ctx, exe, "-test.run=TestNotifyHelperProcess", "--",
			n.Kind.String(), n.Agent, n.Session, file)
	}
}

// TestNotifyHelperProcess is not a test: it is the helper the notifier
// starts below. It appends its notice to the file named by its last
// argument, one "kind|agent|session" line per notice.
func TestNotifyHelperProcess(t *testing.T) {
	i := slices.Index(os.Args, "--")
	if i < 0 {
		t.Skip("helper process only")
	}
	args := os.Args[i+1:]
	if len(args) != 4 {
		t.Fatalf("helper args %q, want kind, agent, session, and file", args)
	}
	f, err := os.OpenFile(args[3], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintln(f, strings.Join(args[:3], "|")); err != nil {
		t.Fatal(err)
	}
}

// read returns the notices in file, or none when the file does not exist.
func read(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// TestNotifyStartsHelper: an event the options turn on starts the helper
// once; an event they turn off starts nothing.
func TestNotifyStartsHelper(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		kind Kind
		want bool
	}{
		{"needs you on", Options{NeedsYou: true}, NeedsYou, true},
		{"needs you off", Options{TurnEnd: true}, NeedsYou, false},
		{"turn end on", Options{TurnEnd: true}, TurnEnd, true},
		{"turn end off", Options{NeedsYou: true}, TurnEnd, false},
		{"both off for a wait", Options{}, NeedsYou, false},
		{"both off for a turn end", Options{}, TurnEnd, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "notice")
			tt.opts.Helper = helper(t, file)
			New(tt.opts).Notify(Notice{Kind: tt.kind, Agent: "Claude Code", Session: "work"})
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) && tt.want && len(read(t, file)) == 0 {
				time.Sleep(10 * time.Millisecond)
			}
			if !tt.want {
				time.Sleep(300 * time.Millisecond)
			}
			got := read(t, file)
			if tt.want && (len(got) != 1 || got[0] != tt.kind.String()+"|Claude Code|work") {
				t.Fatalf("notices %v, want one %q", got, tt.kind.String()+"|Claude Code|work")
			}
			if !tt.want && len(got) != 0 {
				t.Fatalf("notices %v, want none for an event the options turn off", got)
			}
		})
	}
}

// lockedBuffer is a byte buffer that log handlers in other goroutines can
// write.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// TestNotifyHelperMissing: a helper that cannot start does not hold the
// caller, and the notifier logs the first failure once.
func TestNotifyHelperMissing(t *testing.T) {
	var log lockedBuffer
	missing := filepath.Join(t.TempDir(), "no-such-helper")
	n := New(Options{
		NeedsYou: true, TurnEnd: true,
		Log: slog.New(slog.NewTextHandler(&log, nil)),
		Helper: func(ctx context.Context, _ Notice) *exec.Cmd {
			return exec.CommandContext(ctx, missing)
		},
	})
	start := time.Now()
	n.Notify(Notice{Kind: NeedsYou, Agent: "Claude Code", Session: "work"})
	n.Notify(Notice{Kind: TurnEnd, Agent: "Claude Code", Session: "work"})
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Notify held the caller for %v", d)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(log.String(), "desktop notification failed") {
		time.Sleep(5 * time.Millisecond)
	}
	// Let the second run finish too, so a second log would have shown.
	time.Sleep(100 * time.Millisecond)
	if got := strings.Count(log.String(), "desktop notification failed"); got != 1 {
		t.Fatalf("logged %d failures, want 1: %s", got, log.String())
	}
}

// TestMessage: the body names the agent and the session, and nothing else.
func TestMessage(t *testing.T) {
	tests := []struct {
		name string
		n    Notice
		want string
	}{
		{"needs you", Notice{Kind: NeedsYou, Agent: "Claude Code", Session: "api"}, `Claude Code in session "api" needs you`},
		{"turn end", Notice{Kind: TurnEnd, Agent: "Snow chat", Session: "work"}, `Snow chat in session "work" finished`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := message(tt.n)
			if title != "Snow" || body != tt.want {
				t.Fatalf("message %q, %q, want %q, %q", title, body, "Snow", tt.want)
			}
		})
	}
}

// TestKindString names each kind for logs and the test helper.
func TestKindString(t *testing.T) {
	for kind, want := range map[Kind]string{NeedsYou: "needs-you", TurnEnd: "turn-end", Kind(7): "unknown"} {
		if got := kind.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(kind), got, want)
		}
	}
}
