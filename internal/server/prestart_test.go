package server

import (
	"errors"
	"io/fs"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// heldStart holds the next conversation start that is armed until the
// test releases it, so a test can watch the model goroutine meanwhile.
type heldStart struct {
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	id      atomic.Value // the held conversation's ID, once it started
}

func newHeldStart() *heldStart {
	return &heldStart{entered: make(chan struct{}), release: make(chan struct{})}
}

func (h *heldStart) start(cfg agent.Config, meta convstore.Meta) (*agent.Conversation, error) {
	if !h.armed.CompareAndSwap(true, false) {
		return agent.Start(cfg, meta)
	}
	close(h.entered)
	<-h.release
	conv, err := agent.Start(cfg, meta)
	if err == nil {
		h.id.Store(conv.ID())
	}
	return conv, err
}

func (h *heldStart) open() { h.once.Do(func() { close(h.release) }) }

// waitHeld waits until the armed start is held.
func (h *heldStart) waitHeld(t *testing.T) {
	t.Helper()
	select {
	case <-h.entered:
	case <-time.After(apitest.Wait):
		t.Fatal("the conversation start was never held")
	}
}

// newHeldServer is newChatServer with conversation starts that h can hold.
func newHeldServer(t *testing.T) (*Server, *heldStart, *convstore.Store) {
	t.Helper()
	opts, _, store := chatOptions(t)
	h := newHeldStart()
	opts.Chat.start = h.start
	s := New(opts)
	t.Cleanup(s.Close)
	t.Cleanup(h.open) // runs first, so Close never waits on a held start
	return s, h, store
}

// renamed asks c to rename window 0 to name and waits for that layout.
func renamed(t *testing.T, c *apitest.Client, name string) {
	t.Helper()
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: name}})
	nextLayout(t, c, func(l *api.Layout) bool { return windowName(l) == name })
}

// TestNewSessionDoesNotBlockOwner: while a new session's conversation file
// is being created, other clients still get answers.
func TestNewSessionDoesNotBlockOwner(t *testing.T) {
	s, h, _ := newHeldServer(t)
	a := connect(t, s, 100, 30)
	openChat(t, a)

	h.armed.Store(true)
	b := connect(t, s, 100, 30)
	b.Send(&api.Attach{Session: "slow", Create: true, Dir: t.TempDir()})
	h.waitHeld(t)
	renamed(t, a, "answered")

	h.open()
	panes := shownPanes(nextLayout(t, b, func(*api.Layout) bool { return true }))
	if len(panes) != 1 || panes[0].Kind != api.PaneChat {
		t.Fatalf("panes %+v, want one chat", panes)
	}
}

// TestChatSplitDoesNotBlockOwner: while a chat split's conversation file is
// being created, other clients still get answers.
func TestChatSplitDoesNotBlockOwner(t *testing.T) {
	s, h, _ := newHeldServer(t)
	a := connect(t, s, 100, 30)
	chat := openChat(t, a)
	b := connect(t, s, 100, 30)
	b.Send(&api.Attach{Session: "work"})
	nextLayout(t, b, func(*api.Layout) bool { return true })

	h.armed.Store(true)
	b.Send(&api.Command{Op: api.OpSplit, Target: chat, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat,
	}})
	h.waitHeld(t)
	renamed(t, a, "answered")

	h.open()
	l := nextLayout(t, b, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind != api.PaneChat {
			t.Fatalf("panes %+v, want two chats", shownPanes(l))
		}
	}
}

// TestUnusedPrestartReleasesClaim: a conversation started for a session that
// another client created first is closed, and its writer claim released.
func TestUnusedPrestartReleasesClaim(t *testing.T) {
	s, h, store := newHeldServer(t)
	h.armed.Store(true)
	b := connect(t, s, 100, 30)
	b.Send(&api.Attach{Session: "race", Create: true, Dir: t.TempDir()})
	h.waitHeld(t)

	a := connect(t, s, 100, 30)
	a.Send(&api.Attach{Session: "race", Create: true, Dir: t.TempDir()})
	nextLayout(t, a, func(*api.Layout) bool { return true })

	h.open()
	panes := shownPanes(nextLayout(t, b, func(*api.Layout) bool { return true }))
	held, _ := h.id.Load().(string)
	if held == "" {
		t.Fatal("the held conversation did not start")
	}
	if len(panes) != 1 {
		t.Fatalf("panes %+v, want the one chat a created", panes)
	}
	if snap := b.WaitChat(panes[0].ID, func(*api.ChatSnapshot) bool { return true }); snap.Conversation == held {
		t.Fatal("b's pane took the conversation started for the session a created")
	}
	deadline := time.Now().Add(apitest.Wait)
	for {
		_, err := store.Create(convstore.Meta{ID: held})
		if errors.Is(err, fs.ErrExist) {
			return // the claim is free; only the file is left
		}
		if !errors.Is(err, convstore.ErrInUse) {
			t.Fatalf("create %s: %v, want the file to exist", held, err)
		}
		if time.Now().After(deadline) {
			t.Fatal("the unused conversation still holds its writer claim")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
