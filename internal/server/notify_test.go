package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/notify"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/pkg/api"
)

// notifyHelper returns a notifier that starts this test binary as its
// helper, and the file the helper appends each notice to.
func notifyHelper(t *testing.T) (*notify.Notifier, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "notices")
	n := notify.New(notify.Options{
		NeedsYou: true, TurnEnd: true,
		Helper: func(ctx context.Context, n notify.Notice) *exec.Cmd {
			return exec.CommandContext(ctx, exe, "-test.run=TestNotifyHelperProcess", "--",
				n.Kind.String(), n.Agent, n.Session, file)
		},
	})
	return n, file
}

// TestNotifyHelperProcess is not a test: it is the helper the notifier
// starts in the tests below. It appends its notice to the file named by its
// last argument, one "kind|agent|session" line per notice.
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

// notices returns the notices written to file so far. A file that does not
// exist yet holds none.
func notices(t *testing.T, file string) []string {
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

// waitNotices waits until file holds want notices, and returns them.
func waitNotices(t *testing.T, file string, want int) []string {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		if got := notices(t, file); len(got) >= want {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("notices %v, want %d", notices(t, file), want)
	return nil
}

// expectNoNotice fails when file holds anything but count notices soon.
func expectNoNotice(t *testing.T, file string, count int) {
	t.Helper()
	time.Sleep(300 * time.Millisecond)
	if got := notices(t, file); len(got) != count {
		t.Fatalf("notices %v, want %d", got, count)
	}
}

// notifyChat starts a chat server whose notifications go to a new file, and
// opens one chat in session "work".
func notifyChat(t *testing.T, replies ...providertest.Reply) (*Server, *apitest.Client, int, string) {
	t.Helper()
	n, file := notifyHelper(t)
	s, _, c, id := chatInFolderOpts(t, func(o *Options) { o.Notify = n }, t.TempDir(), replies...)
	return s, c, id, file
}

// TestNotifyOnNeedsYou: a chat that starts to wait for the user shows one
// notification that names the agent and the session, and no more.
func TestNotifyOnNeedsYou(t *testing.T) {
	_, c, id, file := notifyChat(t, writeReply("t1"), reply("done"))
	say(c, id, "write it")
	nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou))
	if got := waitNotices(t, file, 1); got[0] != "needs-you|Snow chat|work" {
		t.Fatalf("notice %q, want the chat that needs the user in session work", got[0])
	}
	expectNoNotice(t, file, 1)
}

// TestNotifyDropsQuestionText: a question starts the notification, and its
// text and the message the user sent never reach the notification.
func TestNotifyDropsQuestionText(t *testing.T) {
	_, c, id, file := notifyChat(t, askReply("q1"), reply("done"))
	say(c, id, "pick a store")
	nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou))
	if got := waitNotices(t, file, 1); got[0] != "needs-you|Snow chat|work" {
		t.Fatalf("notice %q, want only the agent and the session", got[0])
	}
}

// TestNotifyOnTurnEnd: every turn end shows one notification, not one per
// change of the agent's text.
func TestNotifyOnTurnEnd(t *testing.T) {
	_, c, id, file := notifyChat(t, reply("one"), reply("two"))
	for i, text := range []string{"first", "second"} {
		say(c, id, text)
		c.WaitChat(id, func(s *api.ChatSnapshot) bool {
			return lastReply(s) == []string{"one", "two"}[i] && idle(s)
		})
		got := waitNotices(t, file, i+1)
		if got[i] != "turn-end|Snow chat|work" {
			t.Fatalf("notice %q, want the chat that finished a turn in session work", got[i])
		}
	}
	expectNoNotice(t, file, 2)
}

// TestNotifySkipsFocusedPane: a client whose terminal has focus on the
// agent's pane already shows the result, so the server sends nothing.
func TestNotifySkipsFocusedPane(t *testing.T) {
	_, c, id, file := notifyChat(t, writeReply("t1"), reply("done"))
	c.Send(&api.Command{Op: api.OpFocus, Target: id})
	l := nextLayout(t, c, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == id {
				return p.Focused
			}
		}
		return false
	})
	if l.Active != 0 {
		t.Fatalf("shown window %d, want the one with pane %d", l.Active, id)
	}
	c.Send(&api.ClientFocus{Focused: true})
	say(c, id, "write it")
	nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou))
	expectNoNotice(t, file, 0)
}

// TestNotifyWhenAnotherSessionHasFocus: a focused client that shows a
// different session does not silence the notification.
func TestNotifyWhenAnotherSessionHasFocus(t *testing.T) {
	s, c, id, file := notifyChat(t, writeReply("t1"), reply("done"))
	other := connect(t, s, 80, 24)
	other.Send(&api.Attach{Session: "other", Create: true})
	nextLayout(t, other, func(*api.Layout) bool { return true })
	other.Send(&api.ClientFocus{Focused: true})
	say(c, id, "write it")
	nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou))
	if got := waitNotices(t, file, 1); got[0] != "needs-you|Snow chat|work" {
		t.Fatalf("notice %q, want the chat in the session the user does not look at", got[0])
	}
}
