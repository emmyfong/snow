package agent

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

// TestFailureMakesNoSecondRequest: after a failed request the agent waits
// for the user. It does not retry, and it does not move to another model.
func TestFailureMakesNoSecondRequest(t *testing.T) {
	for _, kind := range []provider.Kind{provider.KindRateLimited, provider.KindOverloaded, provider.KindNetwork, provider.KindModelNotFound} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			h := newHarness(t, nil, providertest.Reply{StartErr: &provider.Error{Kind: kind}}, text("must not be asked"))
			_ = h.conv.Send("hi")
			if end := next[*TurnEnded](t, h); end.Reason != EndFailed {
				t.Fatalf("ended %+v", end)
			}
			time.Sleep(100 * time.Millisecond) // a retry would be quick; nothing should come
			if n := len(h.fake.Requests()); n != 1 {
				t.Fatalf("%d requests after a failure, want 1", n)
			}
			if got, want := h.fake.Requests()[0].Model, strings.TrimPrefix(h.conv.model, "fake/"); got != want {
				t.Fatalf("model changed from %q to %q", h.conv.model, got)
			}
		})
	}
}
