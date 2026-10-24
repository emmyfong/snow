package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// TestSwitchAppliesNextTurn: a switch during a turn leaves that turn's
// model alone and applies to the next turn.
func TestSwitchAppliesNextTurn(t *testing.T) {
	waiting := providertest.Reply{Events: []provider.Event{&provider.TextDelta{Text: "a"}}, Wait: true}
	h := newHarness(t, nil, waiting, text("b"))
	_ = h.conv.Send("one")
	next[*TextDelta](t, h)
	if err := h.conv.SwitchModel("fake/model-2", "high"); err != nil {
		t.Fatal(err)
	}
	_ = h.conv.Interrupt()
	next[*TurnEnded](t, h)
	if m := next[*ModelChanged](t, h); m.Model != "fake/model-2" || m.Failure != nil {
		t.Fatalf("model changed %+v", m)
	}
	_ = h.conv.Send("two")
	next[*TurnEnded](t, h)
	reqs := h.fake.Requests()
	if len(reqs) != 2 || reqs[0].Model != "model-1" || reqs[1].Model != "model-2" || reqs[1].Effort != "high" {
		t.Fatalf("requests %+v", reqs)
	}
}

// TestResumeUsesRecordedModel: a resumed conversation uses the last model
// in its file. Config has no default model, so nothing else can win.
func TestResumeUsesRecordedModel(t *testing.T) {
	h := newHarness(t, nil, text("x"))
	_ = h.conv.SwitchModel("fake/model-3", "")
	next[*ModelChanged](t, h)
	id := h.conv.ID()
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}
	c, _, err := Resume(h.config(t), id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.Send("hi")
	next[*TurnEnded](t, h)
	if reqs := h.fake.Requests(); len(reqs) != 1 || reqs[0].Model != "model-3" {
		t.Fatalf("requests %+v", reqs)
	}
}

// TestNoFallbackOnError: every typed error stops the turn after one call
// to the requested model, with choices for the user.
func TestNoFallbackOnError(t *testing.T) {
	tests := []struct {
		kind provider.Kind
		want FailureKind
	}{
		{provider.KindBadKey, FailBadKey}, {provider.KindNoKey, FailNoKey},
		{provider.KindModelNotFound, FailModelNotFound}, {provider.KindRateLimited, FailRateLimited},
		{provider.KindOverloaded, FailOverloaded}, {provider.KindContextTooLong, FailContextTooLong},
		{provider.KindNetwork, FailNetwork}, {provider.KindOther, FailOther},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			pe := &provider.Error{Kind: tt.kind, RetryAfter: 5 * time.Second, Err: errors.New("x")}
			h := newHarness(t, nil, providertest.Reply{StartErr: pe}, text("never"))
			_ = h.conv.Send("hi")
			end := next[*TurnEnded](t, h)
			if end.Reason != EndFailed || end.Failure.Kind != tt.want || len(end.Failure.Choices) == 0 {
				t.Fatalf("ended %+v %+v", end, end.Failure)
			}
			if reqs := h.fake.Requests(); len(reqs) != 1 || reqs[0].Model != "model-1" {
				t.Fatalf("requests %+v", reqs)
			}
		})
	}
}

// TestReportedModelDiffers: the usage event holds both model IDs.
func TestReportedModelDiffers(t *testing.T) {
	h := newHarness(t, nil, text("x"))
	_ = h.conv.Send("hi")
	u := next[*Usage](t, h)
	if u.Requested != testModel || u.Reported != "model-1-20261001" {
		t.Fatalf("usage %+v", u)
	}
}

// TestModelNotFoundKeepsRecord: the file's model does not change.
func TestModelNotFoundKeepsRecord(t *testing.T) {
	pe := &provider.Error{Kind: provider.KindModelNotFound}
	h := newHarness(t, nil, providertest.Reply{StartErr: pe})
	_ = h.conv.Send("hi")
	next[*TurnEnded](t, h)
	c, err := h.store.Load(h.conv.ID())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range c.Records {
		if _, ok := r.(*convstore.ModelSwitch); ok {
			t.Fatal("a model switch was recorded")
		}
	}
	if c.Meta.Model != testModel {
		t.Fatalf("meta model %s", c.Meta.Model)
	}
}
