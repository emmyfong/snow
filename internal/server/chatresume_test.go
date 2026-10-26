package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/pkg/api"
)

// saveConversation writes a conversation file in folder with recs and
// returns its ID. Nothing holds it open.
func saveConversation(t *testing.T, store *convstore.Store, folder string, recs ...convstore.Record) string {
	t.Helper()
	w, err := store.Create(convstore.Meta{Folder: folder, Model: chatModel, Created: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if err := w.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.ID()
}

// pastExchange is one saved question and answer.
func pastExchange() []convstore.Record {
	at := time.Now()
	return []convstore.Record{
		&convstore.User{At: at, Text: "hello"},
		&convstore.Assistant{At: at, Stop: convstore.StopEndTurn, Blocks: []convstore.Block{{Kind: convstore.BlockText, Text: "hi"}}},
	}
}

// TestListIsBounded: a long list or a long title never makes a frame too
// large. The newest conversations stay, and the list says how many it left
// out.
func TestListIsBounded(t *testing.T) {
	old := listBudget
	listBudget = 4096
	t.Cleanup(func() { listBudget = old })
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	entries, _ := store.ListAll()
	if len(entries) != 1 {
		t.Fatalf("entries %v", entries)
	}
	w, err := store.Create(convstore.Meta{Folder: entries[0].Folder, Profile: entries[0].Profile, Model: chatModel, Created: time.Now(), Title: strings.Repeat("long ", 5000)})
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	for i := 0; i < 40; i++ {
		saveConversation(t, store, entries[0].Folder)
	}

	c.Send(&api.ConversationListRequest{Pane: id, All: true})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.All })
	if l.Truncated == 0 || len(l.Items)+l.Truncated != 42 {
		t.Fatalf("%d items, %d left out; want 42 in all", len(l.Items), l.Truncated)
	}
	var frame bytes.Buffer
	if err := api.WriteFrame(&frame, l); err != nil {
		t.Fatalf("list does not fit a frame: %v", err)
	}
	for _, it := range l.Items {
		if n := len([]rune(it.Title)); n > maxListText {
			t.Fatalf("a title of %d characters was sent", n)
		}
	}
}

// slowModels is a provider whose model list waits until release closes.
type slowModels struct {
	*providertest.Fake
	release chan struct{}
}

func (p slowModels) Models(ctx context.Context) ([]provider.Model, error) {
	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return p.Fake.Models(ctx)
}

// TestEffortDoesNotBlockTheReader: while /effort waits for a provider's
// model list, the same client's Esc still stops the turn, and a second
// /effort says the list is loading instead of fetching it again.
func TestEffortDoesNotBlockTheReader(t *testing.T) {
	r := reply("working")
	r.Wait = true
	opts, fake, _ := chatOptions(t, r)
	fake.SetModels([]provider.Model{{ID: "model-1", Efforts: []provider.Effort{provider.EffortHigh}}}, nil)
	release := make(chan struct{})
	reg := provider.NewRegistry()
	if err := reg.Register(slowModels{fake, release}); err != nil {
		t.Fatal(err)
	}
	opts.Chat.Providers = reg
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "go")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Running })

	chatInputOf(c, id, api.ChatInputEffort, func(in *api.ChatInput) { in.Effort = "high" })
	chatInputOf(c, id, api.ChatInputEffort, func(in *api.ChatInput) { in.Effort = "high" })
	if e := nextError(t, c); !strings.Contains(e.Message, "loading") {
		t.Fatalf("second /effort: %q", e.Message)
	}
	chatInputOf(c, id, api.ChatInputInterrupt, nil)
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return !s.Running })

	close(release)
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Effort == "high" })
	if n := fake.ModelCalls(); n != 1 {
		t.Fatalf("the provider was asked %d times, want 1", n)
	}
}

// TestNewRefusedRightAfterSend: a /new that comes a moment after a message
// is refused, even before the turn's first event reaches the pane.
func TestNewRefusedRightAfterSend(t *testing.T) {
	const rounds = 10
	replies := make([]providertest.Reply, rounds)
	for i := range replies {
		replies[i] = reply("working")
		replies[i].Wait = true
	}
	s, _, _ := newChatServer(t, replies...)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	conv := c.Chat(id).Conversation
	for i := 0; i < rounds; i++ {
		say(c, id, "go")
		chatInputOf(c, id, api.ChatInputNew, nil)
		if e := nextError(t, c); !strings.Contains(e.Message, "Esc") {
			t.Fatalf("round %d: refusal %q", i, e.Message)
		}
		chatInputOf(c, id, api.ChatInputInterrupt, nil)
		c.WaitChat(id, func(s *api.ChatSnapshot) bool { return idle(s) && s.Turn == i+1 })
	}
	if got := c.Chat(id).Conversation; got != conv {
		t.Fatal("a refused /new changed the conversation")
	}
}

// TestResumeRefusedWhileAnotherWriterHoldsIt: a conversation some other
// writer holds is not opened a second time, and the pane keeps its own.
func TestResumeRefusedWhileAnotherWriterHoldsIt(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	own := c.Chat(id).Conversation
	past := saveConversation(t, store, t.TempDir(), pastExchange()...)
	w, err := store.Writer(past)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	if e := nextError(t, c); !strings.Contains(e.Message, "already open") {
		t.Fatalf("refusal %q", e.Message)
	}
	if got := c.Chat(id).Conversation; got != own {
		t.Fatal("the pane changed its conversation")
	}
}

// TestResumeFolderGone: a conversation whose folder was deleted is refused
// with the folder named, and the pane keeps its own conversation.
func TestResumeFolderGone(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	own := c.Chat(id).Conversation
	gone := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(gone, 0o700); err != nil {
		t.Fatal(err)
	}
	past := saveConversation(t, store, gone, pastExchange()...)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	if e := nextError(t, c); !strings.Contains(e.Message, "is gone") || !strings.Contains(e.Message, gone) {
		t.Fatalf("refusal %q", e.Message)
	}
	if got := c.Chat(id).Conversation; got != own {
		t.Fatal("the pane changed its conversation")
	}
}

// TestResumeFromAnotherFolderAsksAgain: a file saved in auto mode in
// another folder starts in ask, says so, and saves that; the same file in
// the pane's own folder keeps auto (TestResumeLoadsConversation).
func TestResumeFromAnotherFolderAsksAgain(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	other := t.TempDir()
	recs := append(pastExchange(), &convstore.Mode{At: time.Now(), Mode: api.ChatModeAuto})
	past := saveConversation(t, store, other, recs...)

	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	if got.Mode != api.ChatModeAsk {
		t.Fatalf("mode %q, want ask", got.Mode)
	}
	if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, "mode ask") || !strings.Contains(e.Message, other) {
		t.Fatalf("notice %+v", e)
	}
	eventually(t, "the ask record", func() bool {
		var last string
		for _, r := range conversationRecords(t, store, past) {
			if m, ok := r.(*convstore.Mode); ok {
				last = m.Mode
			}
		}
		return last == api.ChatModeAsk
	})
}

// TestResumeShowsAFailedTurnAsFailed: a turn that failed shows the error
// again after a resume, not a stop by the user.
func TestResumeShowsAFailedTurnAsFailed(t *testing.T) {
	pe := &provider.Error{Kind: provider.KindNetwork, Err: errors.New("down")}
	s, _, _ := newChatServer(t, providertest.Reply{StartErr: pe})
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	say(c, first, "hi")
	c.WaitChat(first, func(s *api.ChatSnapshot) bool {
		return idle(s) && len(s.Entries) > 0 && s.Entries[len(s.Entries)-1].Kind == api.ChatEntryError
	})
	past := c.Chat(first).Conversation
	second := openSecondChat(t, c, first)
	c.Send(&api.Command{Op: api.OpClose, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == first {
				return false
			}
		}
		return true
	})

	chatInputOf(c, second, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(second, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	last := got.Entries[len(got.Entries)-1]
	if last.Kind != api.ChatEntryError || last.Error == nil || last.Error.Message == "" {
		t.Fatalf("last entry %+v, want the error", last)
	}
	for _, e := range got.Entries {
		if e.Kind == api.ChatEntryInterrupted {
			t.Fatal("the failed turn shows as stopped")
		}
	}
}

// TestFinishInputTellsOfAReplacedConversation: an input that reached a
// conversation another client replaced gets an answer. One for a pane that
// closed gets none.
func TestFinishInputTellsOfAReplacedConversation(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	var old *agent.Conversation
	var cl *client
	s.call(func(st *state) {
		for k := range st.clients {
			cl = k
			old = st.paneOf(k, id).chat.conv
		}
	})
	chatInputOf(c, id, api.ChatInputNew, nil)
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Agent != string(old.Agent()) })

	s.finishInput(cl, &api.ChatInput{Pane: id}, old, agent.ErrClosed)
	if e := nextError(t, c); !strings.Contains(e.Message, "another conversation") {
		t.Fatalf("answer %q", e.Message)
	}
	s.finishInput(cl, &api.ChatInput{Pane: 999}, old, agent.ErrClosed)
	if m, _ := c.Next(200 * time.Millisecond); m != nil {
		if e, ok := m.(*api.Error); ok {
			t.Fatalf("a closed pane was told %q", e.Message)
		}
	}
}

// TestAwaitRelease: a resume waits for a conversation a closed pane is still
// saving, returns at once for any other, and gives up after releaseWait.
func TestAwaitRelease(t *testing.T) {
	s, _, _ := newChatServer(t)
	took := func(id string) time.Duration {
		start := time.Now()
		s.awaitRelease(id)
		return time.Since(start)
	}
	if d := took("nothing-closes"); d > 50*time.Millisecond {
		t.Fatalf("waited %v for a conversation nothing is closing", d)
	}

	gate := make(chan struct{})
	s.releases.Store("closing", gate)
	time.AfterFunc(200*time.Millisecond, func() { close(gate) })
	if d := took("closing"); d < 150*time.Millisecond {
		t.Fatalf("returned after %v, before the close ended", d)
	}

	old := releaseWait
	releaseWait = 100 * time.Millisecond
	t.Cleanup(func() { releaseWait = old })
	s.releases.Store("stuck", make(chan struct{}))
	if d := took("stuck"); d < 80*time.Millisecond || d > 2*time.Second {
		t.Fatalf("a stuck close made the resume wait %v", d)
	}
}
