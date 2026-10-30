package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// chatDirs are the folders two servers share, as a restart does: the saved
// sessions and the conversations.
type chatDirs struct{ sessions, convs string }

func newChatDirs(t *testing.T) chatDirs {
	t.Helper()
	return chatDirs{sessions: t.TempDir(), convs: filepath.Join(t.TempDir(), "conversations")}
}

// chatServerIn starts a chat server over d, saving sessions at once.
func chatServerIn(t *testing.T, d chatDirs, replies ...providertest.Reply) (*Server, *providertest.Fake, *convstore.Store) {
	t.Helper()
	return chatServerPrepared(t, d, nil, replies...)
}

// chatServerPrepared is chatServerIn that calls prepare with the server's
// conversation store before the server starts. A writer held through that
// store is a writer the server finds in use.
func chatServerPrepared(t *testing.T, d chatDirs, prepare func(*convstore.Store), replies ...providertest.Reply) (*Server, *providertest.Fake, *convstore.Store) {
	t.Helper()
	opts, fake, cs := chatOptionsIn(t, d.convs, replies...)
	if prepare != nil {
		prepare(cs)
	}
	opts.SessionsDir = d.sessions
	opts.saveDelay = 20 * time.Millisecond
	opts.busyEvery = time.Hour
	s := New(opts)
	t.Cleanup(s.Close)
	return s, fake, cs
}

// stopBySignal stops s the way an OS shutdown does, which keeps the saved
// sessions as they are.
func stopBySignal(s *Server) {
	s.keepDiskOnClose()
	s.Close()
}

// chatPaneOf returns the saved chat pane of session name, waiting for the
// saver to write it.
func chatPaneOf(t *testing.T, d chatDirs, name string) store.Pane {
	t.Helper()
	var got store.Pane
	eventually(t, "a saved chat pane", func() bool {
		snap, err := openStore(t, d.sessions).Load(name)
		if err != nil {
			return false
		}
		for _, w := range snap.Windows {
			for _, p := range w.Panes {
				if p.Kind == store.KindChat {
					got = p
					return true
				}
			}
		}
		return false
	})
	return got
}

// TestRestoreChatPane: stop the server by signal, start it again, attach,
// and see the transcript.
func TestRestoreChatPane(t *testing.T) {
	d := newChatDirs(t)
	folder := t.TempDir()
	first, _, _ := chatServerIn(t, d, reply("hi there"))
	c1 := connect(t, first, 100, 30)
	id := attachChat(t, c1, folder)
	say(c1, id, "hello")
	c1.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "hi there" && idle(s) })
	saved := chatPaneOf(t, d, "work")
	if saved.Conversation != c1.Chat(id).Conversation || saved.Folder != folder || len(saved.Scrollback) != 0 {
		t.Fatalf("saved pane %+v, want the chat's conversation and folder, and no scrollback", saved)
	}
	stopBySignal(first)

	second, _, _ := chatServerIn(t, d)
	c2 := connect(t, second, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	l := nextLayout(t, c2, func(l *api.Layout) bool { return chatIn(l) != 0 })
	got := c2.WaitChat(chatIn(l), func(*api.ChatSnapshot) bool { return true })
	if got.Conversation != saved.Conversation || lastReply(got) != "hi there" || !idle(got) {
		t.Fatalf("restored chat %+v, want the saved transcript, idle", got)
	}
	if got.Entries[0].Kind != api.ChatEntryUser || got.Entries[0].Text != "hello" {
		t.Fatalf("first entry %+v, want the user's message", got.Entries[0])
	}
}

// TestRestoreChatBesideShell: a window with a chat and a terminal comes back
// with both, in the layout they had.
func TestRestoreChatBesideShell(t *testing.T) {
	d := newChatDirs(t)
	first, _, _ := chatServerIn(t, d, reply("ok"))
	c1 := connect(t, first, 100, 30)
	openChatBesideShell(t, c1)
	chatPaneOf(t, d, "work")
	stopBySignal(first)

	second, _, _ := chatServerIn(t, d)
	c2 := connect(t, second, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	l := nextLayout(t, c2, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	kinds := map[string]int{}
	for _, p := range shownPanes(l) {
		kinds[p.Kind]++
	}
	if kinds[api.PaneChat] != 1 || kinds[api.PaneTerminal] != 1 {
		t.Fatalf("restored kinds %v, want one chat and one terminal", kinds)
	}
	// The terminal was on the left of the chat.
	if p := shownPanes(l); p[0].Kind != api.PaneTerminal || p[1].Kind != api.PaneChat {
		t.Fatalf("restored order %+v, want the terminal left of the chat", p)
	}
}

// TestRestoreInterruptedTurn: a conversation file that ends in the middle of
// a reply shows an interrupted turn. Snow starts no turn.
func TestRestoreInterruptedTurn(t *testing.T) {
	d := newChatDirs(t)
	folder := t.TempDir()
	cs, err := convstore.Open(d.convs)
	if err != nil {
		t.Fatal(err)
	}
	id := saveConversation(t, cs, folder, &convstore.User{At: time.Now(), Text: "write a poem"})
	writeSnapshot(t, d.sessions, chatSnapshot(id, folder))

	s, fake, _ := chatServerIn(t, d)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work"})
	l := nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 })
	got := c.WaitChat(chatIn(l), func(*api.ChatSnapshot) bool { return true })
	if got.Running || !idle(got) {
		t.Fatalf("restored chat %+v, want it idle", got)
	}
	last := got.Entries[len(got.Entries)-1]
	if last.Kind != api.ChatEntryInterrupted {
		t.Fatalf("last entry %+v, want an interrupted turn", last)
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(fake.Requests()); n != 0 {
		t.Fatalf("the provider got %d requests; a restore must not restart the turn", n)
	}
}

// mustJSON encodes v, which the tests build themselves.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// chatSnapshot is a saved session "work" with one chat pane that shows
// conversation id.
func chatSnapshot(id, folder string) store.Snapshot {
	return store.Snapshot{Name: "work", Active: 1, Windows: []store.Window{{
		Index: 1, Name: "chat", Focused: 1,
		Layout: mustJSON(layout.New(1)),
		Panes:  []store.Pane{{ID: 1, Kind: store.KindChat, Profile: "", Folder: folder, Conversation: id}},
	}}}
}

// TestRestoreMissingConversation: the session restores with an empty chat
// and a notice that names the file. The terminal beside it still comes back.
func TestRestoreMissingConversation(t *testing.T) {
	d := newChatDirs(t)
	folder := t.TempDir()
	snap := chatSnapshot("20200101-000000-deadbeef", folder)
	tree := layout.New(1)
	if err := tree.Split(1, layout.Right, 2); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	snap.Windows[0].Layout = raw
	snap.Windows[0].Panes = append(snap.Windows[0].Panes, store.Pane{ID: 2, Folder: folder})
	writeSnapshot(t, d.sessions, snap)

	s, _, _ := chatServerIn(t, d)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work"})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	got := c.WaitChat(chatIn(l), func(*api.ChatSnapshot) bool { return true })
	if len(got.Entries) != 0 || got.Conversation == "20200101-000000-deadbeef" {
		t.Fatalf("restored chat %+v, want an empty new conversation", got)
	}
	if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, "20200101-000000-deadbeef") {
		t.Fatalf("error %+v, want a notice that names the conversation", e)
	}
}

// TestRestoreOldSnapshot: a file written by the previous format restores
// terminal panes, also on a server that runs chats.
func TestRestoreOldSnapshot(t *testing.T) {
	d := newChatDirs(t)
	old := `{"version":1,"name":"api","saved":"2026-10-09T09:14:00Z","active":1,"windows":[` +
		`{"index":1,"name":"sh","layout":` + string(mustJSON(layout.New(3))) + `,"focused":3,"panes":[{"id":3,"profile":"","folder":"` +
		strings.ReplaceAll(t.TempDir(), `\`, `\\`) + `"}]}]}`
	if err := os.MkdirAll(d.sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.sessions, "api.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _, _ := chatServerIn(t, d)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "api"})
	panes := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneTerminal {
		t.Fatalf("panes %+v, want one terminal", panes)
	}
}

// TestResumeChangesSavedSession: after /resume, the saved session holds the
// new conversation ID.
func TestResumeChangesSavedSession(t *testing.T) {
	d := newChatDirs(t)
	folder := t.TempDir()
	s, _, cs := chatServerIn(t, d)
	c := connect(t, s, 100, 30)
	id := attachChat(t, c, folder)
	before := chatPaneOf(t, d, "work")
	other := saveConversation(t, cs, folder, pastExchange()...)
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = other })
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == other })
	eventually(t, "the session file to name the resumed conversation", func() bool {
		snap, err := openStore(t, d.sessions).Load("work")
		return err == nil && snap.Windows[0].Panes[0].Conversation == other
	})
	if before.Conversation == other {
		t.Fatal("the test resumed the conversation the pane already had")
	}
}

// TestRestoreChatInDefaultProfilesDistro: a chat made in a session that
// names no profile, with a WSL default, comes back in the same folder of the
// distribution, not as a missing folder.
func TestRestoreChatInDefaultProfilesDistro(t *testing.T) {
	d := newChatDirs(t)
	wsl := func(s *Server) {
		s.launch.shell = func(string) (shell.Profile, bool) {
			return shell.Profile{Name: "Ubuntu", Distro: "Ubuntu"}, true
		}
		s.launch.distro = func(profile string) (string, bool) { return "Ubuntu", profile == "Ubuntu" }
	}
	const dir = "/home/snow-test-user/project" // in the distribution, not on this host
	first, _, cs := chatServerIn(t, d)
	wsl(first)
	c1 := connect(t, first, 100, 30)
	c1.Send(&api.Attach{Session: "wsl", Create: true, Dir: dir})
	nextLayout(t, c1, func(l *api.Layout) bool { return chatIn(l) != 0 })
	saved := chatPaneOf(t, d, "wsl")
	// Without this, the restore below passes for the wrong reason: a chat
	// that fell back to the home folder resumes there.
	if loaded, err := cs.Load(saved.Conversation); err != nil || loaded.Meta.Folder != dir {
		t.Fatalf("conversation folder %q, %v; want %q", loaded.Meta.Folder, err, dir)
	}
	stopBySignal(first)

	second, _, _ := chatServerIn(t, d)
	wsl(second)
	c2 := connect(t, second, 100, 30)
	c2.Send(&api.Attach{Session: "wsl"})
	l := nextLayout(t, c2, func(l *api.Layout) bool { return chatIn(l) != 0 })
	got := c2.WaitChat(chatIn(l), func(*api.ChatSnapshot) bool { return true })
	if got.Conversation != saved.Conversation {
		t.Fatalf("restored conversation %q, want %q: the chat did not resume", got.Conversation, saved.Conversation)
	}
}

// savedChatID waits until the saver has written session "work" again after
// since, and returns the conversation its chat pane names.
func savedChatID(t *testing.T, d chatDirs, since time.Time) string {
	t.Helper()
	var id string
	eventually(t, "the session to be saved again", func() bool {
		snap, err := openStore(t, d.sessions).Load("work")
		if err != nil || !snap.Saved.After(since) {
			return false
		}
		id = snap.Windows[0].Panes[0].Conversation
		return true
	})
	return id
}

// TestRestoreFailureKeepsSavedConversation: a conversation that cannot be
// resumed gives an empty chat, but the session file keeps naming the
// conversation until the chat has a message of its own, so a later restart
// can still reach it.
func TestRestoreFailureKeepsSavedConversation(t *testing.T) {
	const gone = "20200101-000000-deadbeef"
	for name, damage := range map[string]func(t *testing.T, cs *convstore.Store){
		"missing": func(*testing.T, *convstore.Store) {},
		"unreadable": func(t *testing.T, cs *convstore.Store) {
			p, err := cs.Path(gone)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("not json\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := newChatDirs(t)
			writeSnapshot(t, d.sessions, chatSnapshot(gone, t.TempDir()))
			since := time.Now()
			s, _, cs := chatServerIn(t, d, reply("ok"))
			damage(t, cs)
			c := connect(t, s, 100, 30)
			c.Send(&api.Attach{Session: "work"})
			id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
			got := c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
			if got.Conversation == gone {
				t.Fatalf("the chat resumed %s", gone)
			}
			if saved := savedChatID(t, d, since); saved != gone {
				t.Fatalf("session names %s, want %s until the chat has a message", saved, gone)
			}

			say(c, id, "hello")
			c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "ok" && idle(s) })
			eventually(t, "the session to name the new conversation", func() bool {
				snap, err := openStore(t, d.sessions).Load("work")
				return err == nil && snap.Windows[0].Panes[0].Conversation == got.Conversation
			})
		})
	}
}

// TestRestoreChatThatCannotStart: when even an empty conversation cannot
// start, the pane opens as a terminal and the user is told why.
func TestRestoreChatThatCannotStart(t *testing.T) {
	d := newChatDirs(t)
	writeSnapshot(t, d.sessions, chatSnapshot("20200101-000000-deadbeef", t.TempDir()))
	s, _, _ := chatServerIn(t, d)
	// A file where the conversation folder was: nothing can be created in it.
	if err := os.RemoveAll(d.convs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.convs, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work"})
	panes := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneTerminal {
		t.Fatalf("panes %+v, want the chat to open as a terminal", panes)
	}
	if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, "20200101-000000-deadbeef") {
		t.Fatalf("error %+v, want a notice that names the conversation", e)
	}
}

// TestRestoreChatFailures: a conversation that cannot be resumed gives an
// empty chat and a notice that says why, whatever the reason, and the session
// keeps naming the conversation.
func TestRestoreChatFailures(t *testing.T) {
	cases := []struct {
		name, notice string
		// prepare returns the conversation the saved pane names.
		prepare func(t *testing.T, cs *convstore.Store) string
	}{
		{"folder gone", "is gone", func(t *testing.T, cs *convstore.Store) string {
			gone := filepath.Join(t.TempDir(), "project")
			if err := os.Mkdir(gone, 0o700); err != nil {
				t.Fatal(err)
			}
			id := saveConversation(t, cs, gone, pastExchange()...)
			if err := os.Remove(gone); err != nil {
				t.Fatal(err)
			}
			return id
		}},
		{"another writer", "another pane", func(t *testing.T, cs *convstore.Store) string {
			id := saveConversation(t, cs, t.TempDir(), pastExchange()...)
			w, err := cs.Writer(id)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close() })
			return id
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d := newChatDirs(t)
			var named string
			since := time.Now()
			s, _, _ := chatServerPrepared(t, d, func(cs *convstore.Store) {
				named = tt.prepare(t, cs)
				loaded, err := cs.Load(named)
				if err != nil {
					t.Fatal(err)
				}
				writeSnapshot(t, d.sessions, chatSnapshot(named, loaded.Meta.Folder))
			})
			c := connect(t, s, 100, 30)
			c.Send(&api.Attach{Session: "work"})
			l := nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 })
			got := c.WaitChat(chatIn(l), func(*api.ChatSnapshot) bool { return true })
			if got.Conversation == named || len(got.Entries) != 0 {
				t.Fatalf("restored chat %+v, want an empty new conversation", got)
			}
			if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, tt.notice) {
				t.Fatalf("error %+v, want a notice with %q", e, tt.notice)
			}
			if saved := savedChatID(t, d, since); saved != named {
				t.Fatalf("session names %s, want %s", saved, named)
			}
		})
	}
}

// TestRestoreChatWithoutChatOptions: a server that runs no chats opens a
// saved chat pane as a terminal, as an old Snow would for a shell.
func TestRestoreChatWithoutChatOptions(t *testing.T) {
	d := newChatDirs(t)
	writeSnapshot(t, d.sessions, chatSnapshot("20200101-000000-deadbeef", t.TempDir()))
	opts := testOptions()
	opts.SessionsDir = d.sessions
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work"})
	panes := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneTerminal {
		t.Fatalf("panes %+v, want one terminal", panes)
	}
}

// TestCloseUnused: an attach that restored nothing lets go of the
// conversations it resumed for nothing, so their files stay free. A
// conversation a pane took stays held, and a chat that failed to start has
// nothing to close.
func TestCloseUnused(t *testing.T) {
	s, _, cs := newChatServer(t)
	start := func() *agent.Conversation {
		t.Helper()
		conv, err := agent.Start(s.chatConfigFor(func() int { return 0 }, "", t.TempDir()), convstore.Meta{Model: chatModel})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conv.Close() })
		return conv
	}
	unused, taken := start(), start()
	s.closeUnused(map[int]*restoredChat{
		1: {conv: unused},
		2: {conv: taken, used: true},
		3: {notice: "a chat that failed to start"},
	})
	if w, err := cs.Writer(unused.ID()); err != nil {
		t.Errorf("the unused conversation is still held: %v", err)
	} else {
		_ = w.Close()
	}
	if _, err := cs.Writer(taken.ID()); !errors.Is(err, convstore.ErrInUse) {
		t.Errorf("the conversation a pane took was let go: %v", err)
	}
}

// TestNewChangesSavedSession: after /new, the saved session holds the new
// conversation ID.
func TestNewChangesSavedSession(t *testing.T) {
	d := newChatDirs(t)
	s, _, _ := chatServerIn(t, d)
	c := connect(t, s, 100, 30)
	id := attachChat(t, c, t.TempDir())
	before := chatPaneOf(t, d, "work")
	chatInputOf(c, id, api.ChatInputNew, nil)
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation != before.Conversation })
	eventually(t, "the session file to name the new conversation", func() bool {
		snap, err := openStore(t, d.sessions).Load("work")
		return err == nil && snap.Windows[0].Panes[0].Conversation == got.Conversation
	})
}
