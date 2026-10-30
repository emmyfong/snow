package server

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// waitModels waits for a ModelList that ok accepts.
func waitModels(t *testing.T, c *apitest.Client, ok func(*api.ModelList) bool) *api.ModelList {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(100 * time.Millisecond)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatalf("waiting for a model list: %v", err)
		}
		if l, isList := m.(*api.ModelList); isList && ok(l) {
			return l
		}
	}
	t.Fatal("no matching ModelList")
	return nil
}

func listModels(l *api.ModelList) []string {
	var out []string
	for _, p := range l.Providers {
		for _, m := range p.Models {
			out = append(out, p.Name+"/"+m.ID)
		}
	}
	return out
}

// TestModelListCache: the first reply comes from the empty cache and a
// later one carries the fetched list; a fresh list is reused, and an old
// one is fetched again.
func TestModelListCache(t *testing.T) {
	opts, fake, _ := chatOptions(t)
	var clock atomic.Int64
	clock.Store(1_000)
	opts.Chat.Now = func() time.Time { return time.Unix(clock.Load(), 0) }
	opts.Chat.ModelTTL = time.Hour
	fake.SetModels([]provider.Model{{
		ID: "model-1", DisplayName: "Model One", ContextWindow: 1_000,
		Efforts: []provider.Effort{provider.EffortHigh},
	}}, nil)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.ModelListRequest{})
	first := waitModels(t, c, func(*api.ModelList) bool { return true })
	if p := first.Providers[0]; p.Name != "fake" || len(p.Models) != 0 || !p.Refreshing {
		t.Fatalf("cache reply %+v", first.Providers)
	}
	full := waitModels(t, c, func(l *api.ModelList) bool { return len(l.Providers[0].Models) == 1 })
	p := full.Providers[0]
	if p.Refreshing || len(p.Err) != 0 || p.Models[0].ID != "model-1" || p.Models[0].ContextWindow != 1_000 {
		t.Fatalf("fetched list %+v", p)
	}
	if len(p.Models[0].Efforts) != 1 || p.Models[0].Efforts[0] != "high" {
		t.Fatalf("efforts %v", p.Models[0].Efforts)
	}
	if n := fake.ModelCalls(); n != 1 {
		t.Fatalf("Models calls = %d, want 1", n)
	}

	// Inside the TTL the cache answers without a new fetch.
	c.Send(&api.ModelListRequest{})
	fresh := waitModels(t, c, func(l *api.ModelList) bool { return !l.Providers[0].Refreshing })
	if got := listModels(fresh); len(got) != 1 || got[0] != "fake/model-1" {
		t.Fatalf("fresh cache %v", got)
	}
	if n := fake.ModelCalls(); n != 1 {
		t.Fatalf("Models calls = %d after a fresh request, want 1", n)
	}

	// Past the TTL the server fetches again.
	clock.Add(2 * 3600)
	c.Send(&api.ModelListRequest{})
	stale := waitModels(t, c, func(l *api.ModelList) bool { return l.Providers[0].Refreshing })
	if got := listModels(stale); len(got) != 1 {
		t.Fatalf("stale cache %v", got)
	}
	waitModels(t, c, func(l *api.ModelList) bool { return !l.Providers[0].Refreshing })
	if n := fake.ModelCalls(); n != 2 {
		t.Fatalf("Models calls = %d after the TTL, want 2", n)
	}
}

// TestModelListFailureKeepsTheList: a failed refresh serves the list it had
// with the error, and does not ask again within the TTL.
func TestModelListFailureKeepsTheList(t *testing.T) {
	opts, fake, _ := chatOptions(t)
	var clock atomic.Int64
	clock.Store(1_000)
	opts.Chat.Now = func() time.Time { return time.Unix(clock.Load(), 0) }
	opts.Chat.ModelTTL = time.Hour
	fake.SetModels([]provider.Model{{ID: "model-1"}}, nil)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.ModelListRequest{})
	waitModels(t, c, func(l *api.ModelList) bool { return len(l.Providers[0].Models) == 1 })

	fake.SetModels(nil, errors.New("provider is down"))
	clock.Add(2 * 3600)
	c.Send(&api.ModelListRequest{})
	failed := waitModels(t, c, func(l *api.ModelList) bool { return l.Providers[0].Err != "" })
	if p := failed.Providers[0]; len(p.Models) != 1 || !strings.Contains(p.Err, "provider is down") {
		t.Fatalf("failed refresh %+v", p)
	}
	if n := fake.ModelCalls(); n != 2 {
		t.Fatalf("Models calls = %d, want 2", n)
	}

	// The failure is cached too, so the broken provider is not asked on
	// every request.
	clock.Add(30 * 60)
	c.Send(&api.ModelListRequest{})
	cached := waitModels(t, c, func(*api.ModelList) bool { return true })
	if p := cached.Providers[0]; len(p.Models) != 1 || p.Err == "" || p.Refreshing {
		t.Fatalf("cached failure %+v", p)
	}
	if n := fake.ModelCalls(); n != 2 {
		t.Fatalf("Models calls = %d within the TTL, want 2", n)
	}

	// Force asks again, even inside the TTL.
	fake.SetModels([]provider.Model{{ID: "model-2"}}, nil)
	c.Send(&api.ModelListRequest{Force: true})
	waitModels(t, c, func(l *api.ModelList) bool { return l.Providers[0].Refreshing })
	forced := waitModels(t, c, func(l *api.ModelList) bool {
		p := l.Providers[0]
		return !p.Refreshing && p.Err == "" && len(p.Models) == 1 && p.Models[0].ID == "model-2"
	})
	if got := listModels(forced); len(got) != 1 || got[0] != "fake/model-2" {
		t.Fatalf("forced refresh %v", got)
	}
}

// TestSwitchFromPicker: Enter on a model switches the chat, and the next
// turn asks the provider for the new model.
func TestSwitchFromPicker(t *testing.T) {
	opts, fake, _ := chatOptions(t, reply("one"), reply("two"))
	fake.SetModels([]provider.Model{{ID: "model-1"}, {ID: "model-2"}}, nil)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)

	c.Send(&api.ModelListRequest{})
	waitModels(t, c, func(l *api.ModelList) bool { return len(l.Providers[0].Models) == 2 })
	c.Send(&api.ChatInput{Pane: id, Agent: c.Chat(id).Agent, Kind: api.ChatInputModel,
		Model: "fake/model-2"})
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Model == "fake/model-2" })

	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "one" && idle(s) })
	reqs := fake.Requests()
	if len(reqs) != 1 || reqs[0].Model != "model-2" {
		t.Fatalf("requests %+v, want one to model-2", reqs)
	}
}

// TestPickModelSavesDefault: the picker's choice switches the chat, ctrl+d
// writes it as the default, and the next chat starts with it.
func TestPickModelSavesDefault(t *testing.T) {
	opts, _, _ := chatOptions(t)
	saved := make(chan [2]string, 1)
	opts.Chat.SaveDefault = func(model, effort string) error {
		saved <- [2]string{model, effort}
		return nil
	}
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	if got := c.Chat(id).Model; got != chatModel {
		t.Fatalf("chat starts with %q, want %q", got, chatModel)
	}
	agent := c.Chat(id).Agent
	c.Send(&api.ChatInput{Pane: id, Agent: agent, Kind: api.ChatInputModel,
		Model: "fake/model-2", Effort: "high", SaveDefault: true})
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Model == "fake/model-2" && s.Effort == "high" })
	select {
	case got := <-saved:
		if got != [2]string{"fake/model-2", "high"} {
			t.Fatalf("saved %v", got)
		}
	case <-time.After(apitest.Wait):
		t.Fatal("SaveDefault was not called")
	}

	// A chat opened after the save starts with the new default.
	c.Send(&api.Command{Op: api.OpSplit, Target: id, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgKind: api.PaneChat}})
	l := nextLayout(t, c, func(l *api.Layout) bool {
		n := 0
		for _, p := range shownPanes(l) {
			if p.Kind == api.PaneChat {
				n++
			}
		}
		return n == 2
	})
	second := 0
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneChat && p.ID != id {
			second = p.ID
		}
	}
	snap := c.WaitChat(second, func(*api.ChatSnapshot) bool { return true })
	if snap.Model != "fake/model-2" || snap.Effort != "high" {
		t.Fatalf("new chat starts with %q %q", snap.Model, snap.Effort)
	}
}

// TestChatWithoutModelRefusesMessages: a chat with no model takes a model
// switch, but a message waits until it has one.
func TestChatWithoutModelRefusesMessages(t *testing.T) {
	opts, fake, _ := chatOptions(t, reply("ok"))
	opts.Chat.DefaultModel = ""
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	if got := c.Chat(id).Model; got != "" {
		t.Fatalf("chat starts with %q, want no model", got)
	}
	agent := c.Chat(id).Agent
	c.Send(&api.ChatInput{Pane: id, Agent: agent, Kind: api.ChatInputMessage, Text: "hi"})
	deadline := time.Now().Add(apitest.Wait)
	for {
		m, err := c.Next(time.Second)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if e, ok := m.(*api.Error); ok {
			if !strings.Contains(e.Message, "no model") {
				t.Fatalf("error %q, want it to name the missing model", e.Message)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the message was not refused")
		}
	}
	if reqs := fake.Requests(); len(reqs) != 0 {
		t.Fatalf("a turn ran without a model: %+v", reqs)
	}

	c.Send(&api.ChatInput{Pane: id, Agent: agent, Kind: api.ChatInputModel, Model: "fake/model-1"})
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Model == "fake/model-1" })
	say(c, id, "hi")
	if got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "ok" && idle(s) }); got.Model != "fake/model-1" {
		t.Fatalf("chat after the switch: %+v", got)
	}
}
