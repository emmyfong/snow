package agent

import (
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// closeAndLoad ends h's conversation and returns what its file holds.
func closeAndLoad(t *testing.T, h *harness) *convstore.Conversation {
	t.Helper()
	id := h.conv.ID()
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := h.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

// TestResumeNumbersTurnsAfterTheSavedOnes: the first turn after a resume
// has the number after the saved turns, so no two turns share a number.
func TestResumeNumbersTurnsAfterTheSavedOnes(t *testing.T) {
	h := newHarness(t, nil, text("one"), text("two"), text("three"))
	_ = h.conv.Send("a")
	next[*TurnEnded](t, h)
	_ = h.conv.Send("b")
	next[*TurnEnded](t, h)
	loaded := closeAndLoad(t, h)

	c, _, err := ResumeLoaded(h.config(t), loaded)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.Send("c")
	if got := next[*TurnStarted](t, h); got.Turn != 3 {
		t.Fatalf("first turn after the resume is %d, want 3", got.Turn)
	}
}

// TestResumeReadsTheFileAfterTheClaim: records saved after the caller's
// read, by the writer that held the file, are in what ResumeLoaded returns
// and in the history the next request sends.
func TestResumeReadsTheFileAfterTheClaim(t *testing.T) {
	h := newHarness(t, nil, text("one"), text("two"))
	_ = h.conv.Send("a")
	next[*TurnEnded](t, h)
	stale, err := h.store.Load(h.conv.ID()) // read while the writer is live
	if err != nil {
		t.Fatal(err)
	}
	_ = h.conv.Send("b") // saved after the read above
	next[*TurnEnded](t, h)
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}

	c, fresh, err := ResumeLoaded(h.config(t), stale)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if len(fresh.Records) <= len(stale.Records) {
		t.Fatalf("returned %d records, the stale read had %d", len(fresh.Records), len(stale.Records))
	}
	if want := len(history(fresh.Records)); len(c.history) != want {
		t.Fatalf("history has %d messages, the file holds %d", len(c.history), want)
	}
}

// TestFailedTurnIsRecordedAsFailed: a turn that fails saves an interruption
// marked Failed, so a resumed view can show the error; a stop by the user
// does not.
func TestFailedTurnIsRecordedAsFailed(t *testing.T) {
	pe := &provider.Error{Kind: provider.KindNetwork, Err: errors.New("down")}
	h := newHarness(t, nil, providertest.Reply{StartErr: pe})
	_ = h.conv.Send("a")
	next[*TurnEnded](t, h)
	var got *convstore.Interruption
	for _, r := range closeAndLoad(t, h).Records {
		if i, ok := r.(*convstore.Interruption); ok {
			got = i
		}
	}
	if got == nil || !got.Failed || got.Reason == "" {
		t.Fatalf("interruption %+v", got)
	}
}

// TestIdleCountsAMessageJustSent: a message sent before the call makes the
// conversation busy at once, with no wait for the turn's first event.
func TestIdleCountsAMessageJustSent(t *testing.T) {
	hold := make(chan struct{})
	r := text("ok")
	r.Hold = hold
	h := newHarness(t, nil, r)
	if !h.conv.Idle() {
		t.Fatal("a new conversation is not idle")
	}
	_ = h.conv.Send("hi")
	if h.conv.Idle() {
		t.Fatal("idle right after a message was sent")
	}
	close(hold)
	next[*TurnEnded](t, h)
	h.waitIdle(t)
	_ = h.conv.Close()
	if h.conv.Idle() {
		t.Fatal("a closed conversation is idle")
	}
}
