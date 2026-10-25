package providertest

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
)

// TestStreamFake: the fake plays a whole reply in order, then io.EOF.
func TestStreamFake(t *testing.T) {
	want := []provider.Event{
		&provider.Start{Model: "m-1"},
		&provider.ThinkingDelta{Text: "hmm"},
		&provider.TextDelta{Text: "Hello"},
		&provider.ToolUse{ID: "t1", Name: "read", Input: []byte(`{"path":"a"}`)},
		&provider.Usage{Input: 10, Output: 5},
		&provider.Stop{Reason: provider.StopToolUse},
	}
	f := New("fake", Reply{Events: want})
	s, err := f.Stream(context.Background(), provider.Request{Model: "m-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	var got []provider.Event
	for {
		ev, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ev)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if r := f.Requests(); len(r) != 1 || r[0].Model != "m-1" {
		t.Fatalf("requests = %v", r)
	}
	if _, err := f.Stream(context.Background(), provider.Request{}); !errors.Is(err, ErrNoReply) {
		t.Fatalf("second Stream error = %v, want ErrNoReply", err)
	}
}

// TestStreamCancel: canceling the context ends the stream with the
// context's error.
func TestStreamCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := New("fake", Reply{Events: []provider.Event{&provider.TextDelta{Text: "a"}, &provider.TextDelta{Text: "b"}}})
	s, err := f.Stream(ctx, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := s.Next(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Next after cancel = %v, want context.Canceled", err)
	}
}

// TestStreamError: a scripted error ends the stream after its events.
func TestStreamError(t *testing.T) {
	boom := &provider.Error{Kind: provider.KindOverloaded}
	f := New("fake", Reply{Events: []provider.Event{&provider.TextDelta{Text: "a"}}, Err: boom}, Reply{StartErr: boom})
	s, _ := f.Stream(context.Background(), provider.Request{})
	_, _ = s.Next()
	if _, err := s.Next(); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the scripted one", err)
	}
	if _, err := f.Stream(context.Background(), provider.Request{}); !errors.Is(err, boom) {
		t.Fatalf("start error = %v, want the scripted one", err)
	}
}

// TestFakeWait: a waiting reply yields its events, then ends only when its
// context does.
func TestFakeWait(t *testing.T) {
	f := New("fake", Reply{Events: []provider.Event{&provider.TextDelta{Text: "hi"}}, Wait: true})
	ctx, cancel := context.WithCancel(context.Background())
	s, err := f.Stream(ctx, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if ev, err := s.Next(); err != nil || ev.(*provider.TextDelta).Text != "hi" {
		t.Fatalf("first event %v, %v", ev, err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.Next()
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Next returned %v before the cancel", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("after cancel: %v", err)
	}
}

// TestStreamHold: a held reply gives its first event, waits for Hold, then
// gives the rest.
func TestStreamHold(t *testing.T) {
	hold := make(chan struct{})
	f := New("fake", Reply{Events: []provider.Event{&provider.TextDelta{Text: "a"}, &provider.TextDelta{Text: "b"}}, Hold: hold})
	s, err := f.Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if ev, err := s.Next(); err != nil || ev.(*provider.TextDelta).Text != "a" {
		t.Fatalf("first: %v %v", ev, err)
	}
	got := make(chan provider.Event, 1)
	go func() {
		ev, _ := s.Next()
		got <- ev
	}()
	select {
	case ev := <-got:
		t.Fatalf("got %v before the hold ended", ev)
	case <-time.After(50 * time.Millisecond):
	}
	close(hold)
	if ev := <-got; ev.(*provider.TextDelta).Text != "b" {
		t.Fatalf("second: %v", ev)
	}
}
