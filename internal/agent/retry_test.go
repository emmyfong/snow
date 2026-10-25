package agent

import (
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// userTexts returns the text of each user message of req, in order.
func userTexts(req provider.Request) []string {
	var out []string
	for _, m := range req.Messages {
		if m.Role != provider.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if t, ok := b.(*provider.Text); ok {
				out = append(out, t.Text)
			}
		}
	}
	return out
}

// TestRetryAfterStartError: the request failed before any reply, so a
// retry sends the same conversation, with no new message.
func TestRetryAfterStartError(t *testing.T) {
	pe := &provider.Error{Kind: provider.KindOverloaded}
	h := newHarness(t, nil, providertest.Reply{StartErr: pe}, text("ok"))
	_ = h.conv.Send("hi")
	if start := next[*TurnStarted](t, h); start.Text != "hi" {
		t.Fatalf("started %+v", start)
	}
	next[*TurnEnded](t, h)
	if err := h.conv.Retry(); err != nil {
		t.Fatal(err)
	}
	if start := next[*TurnStarted](t, h); start.Text != "" {
		t.Fatalf("retry started %+v", start)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndDone {
		t.Fatalf("retry ended %+v", end)
	}
	reqs := h.fake.Requests()
	if len(reqs) != 2 || len(userTexts(reqs[1])) != 1 || userTexts(reqs[1])[0] != "hi" {
		t.Fatalf("requests %+v", reqs)
	}
}

// TestRetryAfterCutReply: the reply was cut off and saved, so a retry
// sends the user's message again after it.
func TestRetryAfterCutReply(t *testing.T) {
	pe := &provider.Error{Kind: provider.KindNetwork}
	cut := providertest.Reply{Events: []provider.Event{&provider.Start{Model: "m"}, &provider.TextDelta{Text: "par"}}, Err: pe}
	h := newHarness(t, nil, cut, text("ok"))
	_ = h.conv.Send("hi")
	next[*TurnEnded](t, h)
	if err := h.conv.Retry(); err != nil {
		t.Fatal(err)
	}
	if start := next[*TurnStarted](t, h); start.Text != "hi" {
		t.Fatalf("retry started %+v", start)
	}
	next[*TurnEnded](t, h)
	reqs := h.fake.Requests()
	if got := userTexts(reqs[len(reqs)-1]); len(got) != 2 || got[1] != "hi" {
		t.Fatalf("user messages %q", got)
	}
}

// TestRetryWhileBusy: a retry during a turn is refused.
func TestRetryWhileBusy(t *testing.T) {
	h := newHarness(t, nil, providertest.Reply{Wait: true})
	_ = h.conv.Send("hi")
	next[*TurnStarted](t, h)
	if err := h.conv.Retry(); !errors.Is(err, ErrBusy) {
		t.Fatalf("retry: %v", err)
	}
}

// TestRetryAfterUserSaveFailure: the user's message could not be saved,
// so it never reached the history. A retry tries that message again,
// not an older one or none, and still sends nothing it cannot save.
func TestRetryAfterUserSaveFailure(t *testing.T) {
	h := newHarness(t, nil, text("never"))
	if err := h.conv.w.Close(); err != nil {
		t.Fatal(err)
	}
	_ = h.conv.Send("hi")
	next[*TurnEnded](t, h)
	if err := h.conv.Retry(); err != nil {
		t.Fatal(err)
	}
	if start := next[*TurnStarted](t, h); start.Text != "hi" {
		t.Fatalf("retry started %+v, want the unsaved message", start)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndFailed || end.Failure == nil || end.Failure.Kind != FailSave {
		t.Fatalf("retry ended %+v", end)
	}
	if n := len(h.fake.Requests()); n != 0 {
		t.Fatalf("the provider got %d requests", n)
	}
}
