package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

// windowName returns the name of the window l shows.
func windowName(l *api.Layout) string {
	for _, w := range l.Windows {
		if w.Index == l.Active {
			return w.Name
		}
	}
	return ""
}

// TestNewSessionOpensChat: the first pane of a new session is a chat, and
// its conversation is saved for the folder the client ran snow from.
func TestNewSessionOpensChat(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	dir := t.TempDir()
	c.Send(&api.Attach{Session: "new", Create: true, Dir: dir})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	panes := shownPanes(l)
	if len(panes) != 1 || panes[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want one chat", panes)
	}
	snap := c.WaitChat(panes[0].ID, func(*api.ChatSnapshot) bool { return true })
	loaded, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Folder != dir {
		t.Errorf("conversation folder %q, want %q", loaded.Meta.Folder, dir)
	}
	if got := windowName(l); got != "chat" {
		t.Errorf("window name %q, want chat", got)
	}
}

// TestNewSessionBadDir: a folder the server cannot use falls back, and the
// chat still opens.
func TestNewSessionBadDir(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "lost", Create: true, Dir: filepath.Join(os.TempDir(), "snow-no-such-folder")})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	panes := shownPanes(l)
	if len(panes) != 1 || panes[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want one chat", panes)
	}
	snap := c.WaitChat(panes[0].ID, func(*api.ChatSnapshot) bool { return true })
	loaded, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(loaded.Meta.Folder) {
		t.Errorf("conversation folder %q is not a usable folder", loaded.Meta.Folder)
	}
}

// TestNewSessionWithoutChat: a server with no chat options keeps a shell as
// the first pane.
func TestNewSessionWithoutChat(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "plain", Create: true})
	panes := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneTerminal {
		t.Fatalf("panes %+v, want one terminal", panes)
	}
}

// TestSplitStartsTerminal: after the first chat, a split opens a terminal
// pane, as before.
func TestSplitStartsTerminal(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "new", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{api.ArgDir: api.DirRight}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	kinds := map[string]int{}
	for _, p := range shownPanes(l) {
		kinds[p.Kind]++
	}
	if kinds[api.PaneChat] != 1 || kinds[api.PaneTerminal] != 1 {
		t.Fatalf("kinds %v, want one chat and one terminal", kinds)
	}
}

// TestChatFolderFromFocusedPane: a chat made in a running session works in
// the folder of the pane it was made from.
func TestChatFolderFromFocusedPane(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	first, other := t.TempDir(), t.TempDir()
	c.Send(&api.Attach{Session: "new", Create: true, Dir: first})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgFolder: other}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	var term api.PaneInfo
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneTerminal {
			term = p
		}
	}
	c.Send(&api.Command{Op: api.OpSplit, Target: term.ID, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgKind: api.PaneChat}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 3 })
	var made int
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneChat && p.ID > term.ID {
			made = p.ID
		}
	}
	snap := c.WaitChat(made, func(*api.ChatSnapshot) bool { return true })
	loaded, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Folder != other {
		t.Errorf("conversation folder %q, want the terminal's %q", loaded.Meta.Folder, other)
	}
}

// TestChatRecordsProfileWithPath: a conversation records the profile its
// commands run in and the folder, so a WSL folder and its Windows view stay
// different folders in the history.
func TestChatRecordsProfileWithPath(t *testing.T) {
	opts, _, store := chatOptions(t)
	opts.Chat.Shell = &shell.Profile{Name: "pwsh", Kind: shell.KindPwsh, Path: "pwsh"}
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	dir := t.TempDir()
	c.Send(&api.Attach{Session: "new", Create: true, Dir: dir})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	snap := c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	loaded, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Profile != "pwsh" || loaded.Meta.Folder != dir {
		t.Errorf("recorded profile %q folder %q, want pwsh and %q", loaded.Meta.Profile, loaded.Meta.Folder, dir)
	}
}

// TestListHidesEmptyConversations: a chat that has no message yet has a
// conversation file, but it is not a conversation to pick up, so the list
// leaves it out. A file that cannot be read is still listed, so the user can
// see it.
func TestListHidesEmptyConversations(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	dir := t.TempDir()
	c.Send(&api.Attach{Session: "new", Create: true, Dir: dir})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	snap := c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	own, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	past := saveConversation(t, store, own.Meta.Folder, pastExchange()...)
	broken, err := store.Path("20200101-000000-badbad01")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c.Send(&api.ConversationListRequest{Pane: id, All: true})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.All })
	got := map[string]bool{}
	for _, it := range l.Items {
		got[it.ID] = true
	}
	if got[snap.Conversation] || !got[past] || !got["20200101-000000-badbad01"] {
		t.Fatalf("listed %v, want the past conversation and the broken file, not the empty chat %s", got, snap.Conversation)
	}
	if l.Truncated != 0 {
		t.Fatalf("truncated %d, want 0: hidden conversations are not left out for room", l.Truncated)
	}
}

// TestFirstChatKeepsTheDefaultProfilesDistro: when the default profile is a
// WSL distribution, the first chat works in a folder of that distribution.
// The folder and the shell must come from the same profile, though the
// session names none.
func TestFirstChatKeepsTheDefaultProfilesDistro(t *testing.T) {
	s, _, store := newChatServer(t)
	s.launch.shell = func(string) (shell.Profile, bool) {
		return shell.Profile{Name: "Ubuntu", Distro: "Ubuntu"}, true
	}
	s.launch.distro = func(profile string) (string, bool) { return "Ubuntu", profile == "Ubuntu" }
	c := connect(t, s, 100, 30)
	const dir = "/home/snow-test-user/project" // in the distribution, not on this host
	c.Send(&api.Attach{Session: "wsl", Create: true, Dir: dir})
	panes := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want one chat", panes)
	}
	snap := c.WaitChat(panes[0].ID, func(*api.ChatSnapshot) bool { return true })
	loaded, err := store.Load(snap.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Folder != dir || loaded.Meta.Profile != "Ubuntu" {
		t.Errorf("conversation in %q with profile %q, want %q with Ubuntu", loaded.Meta.Folder, loaded.Meta.Profile, dir)
	}
}

// TestResumeInDefaultProfilesDistro: /resume in the first chat of a WSL
// default takes a conversation saved in a folder of the distribution.
func TestResumeInDefaultProfilesDistro(t *testing.T) {
	s, _, store := newChatServer(t)
	s.launch.shell = func(string) (shell.Profile, bool) {
		return shell.Profile{Name: "Ubuntu", Distro: "Ubuntu"}, true
	}
	s.launch.distro = func(profile string) (string, bool) { return "Ubuntu", profile == "Ubuntu" }
	c := connect(t, s, 100, 30)
	const dir = "/home/snow-test-user/project" // in the distribution, not on this host
	past := saveConversation(t, store, dir, pastExchange()...)
	c.Send(&api.Attach{Session: "wsl", Create: true, Dir: dir})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
}
