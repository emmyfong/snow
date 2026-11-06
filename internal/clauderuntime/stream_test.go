package clauderuntime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/clauderuntime/runtimetest"
	"github.com/emmyfong/snow/internal/shell"
)

func startTestStream(t *testing.T, mode string) *Stream {
	t.Helper()
	s, err := StartStream(testLaunch(t, mode))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

func testLaunch(t *testing.T, mode string) shell.Launch {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return shell.Launch{Path: exe, Args: []string{"-test.run=^TestStreamHelper$"},
		Env: []string{"SNOW_CLAUDE_STREAM_TEST=" + mode}}
}

func nextFrame(t *testing.T, s *Stream) Frame {
	t.Helper()
	select {
	case frame, ok := <-s.Frames():
		if !ok {
			t.Fatal("runtime stream ended before frame")
		}
		return frame
	case <-time.After(10 * time.Second):
		t.Fatal("runtime frame did not arrive")
		return Frame{}
	}
}

func waitStream(t *testing.T, s *Stream) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("runtime process did not stop")
	}
}

func TestStreamConcurrentRoundTrip(t *testing.T) {
	s := startTestStream(t, "echo")
	if got := nextFrame(t, s); got.Type != "system" || got.SessionID != "test-session" {
		t.Fatalf("initialization identity: %+v", got)
	}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.Send(ctx, map[string]any{"type": "user", "request_id": fmt.Sprint(i), "content": "a\nb"}); err != nil {
				t.Errorf("send: %v", err)
			}
		})
	}
	wg.Wait()
	seen := map[string]bool{}
	for range 12 {
		frame := nextFrame(t, s)
		if frame.Type != "user" || frame.RequestID == "" || seen[frame.RequestID] {
			t.Fatalf("lost, duplicate or interleaved frame: %+v", frame)
		}
		seen[frame.RequestID] = true
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("repeated stop: %v", err)
	}
	if err := s.Send(context.Background(), map[string]string{"type": "user"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("send after stop: %v", err)
	}
}

func TestStreamProtocolFailure(t *testing.T) {
	for _, mode := range []string{"malformed", "missing-type", "oversized", "exit"} {
		t.Run(mode, func(t *testing.T) {
			s := startTestStream(t, mode)
			waitStream(t, s)
			want := ErrProtocol
			if mode == "exit" {
				want = ErrClosed
			}
			if !errors.Is(s.Err(), want) {
				t.Fatalf("terminal error %v, want %v", s.Err(), want)
			}
			if strings.Contains(s.Err().Error(), "private-input") {
				t.Fatal("protocol failure retained raw private data")
			}
		})
	}
}

func TestStreamSlowConsumerRemainsStoppable(t *testing.T) {
	s := startTestStream(t, "flood")
	deadline := time.Now().Add(10 * time.Second)
	for len(s.frames) < cap(s.frames) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(s.frames) != cap(s.frames) {
		t.Fatal("fixture did not fill the bounded frame queue")
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop could not release a full frame queue: %v", err)
	}
}

func TestStreamRejectsUnsentInput(t *testing.T) {
	s := startTestStream(t, "echo")
	nextFrame(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Send(ctx, map[string]string{"type": "user"}); !errors.Is(err, context.Canceled) || errors.Is(err, ErrDelivery) {
		t.Fatalf("already cancelled input has uncertain delivery: %v", err)
	}
	if err := s.Send(context.Background(), strings.Repeat("x", MaxFrameBytes)); !errors.Is(err, ErrProtocol) {
		t.Fatalf("oversized input: %v", err)
	}
	if err := s.Send(context.Background(), map[string]string{"type": "user", "request_id": "only-sent"}); err != nil {
		t.Fatal(err)
	}
	if frame := nextFrame(t, s); frame.RequestID != "only-sent" {
		t.Fatalf("rejected input reached the runtime: %+v", frame)
	}
}

func TestStreamBlockedInputIsCancellable(t *testing.T) {
	s := startTestStream(t, "no-read")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- s.Send(ctx, map[string]string{"type": "user", "content": strings.Repeat("x", 2<<20)})
	}()
	// The helper acknowledges the first byte, then stops reading. Cancel only
	// after delivery has actually begun, independently of scheduler speed.
	nextFrame(t, s)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrDelivery) || !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked delivery: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancellation did not release sender")
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop while writing: %v", err)
	}
}

func TestStreamHelper(_ *testing.T) {
	mode := os.Getenv("SNOW_CLAUDE_STREAM_TEST")
	if mode == "" {
		return
	}
	write := func(line string) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			os.Exit(2)
		}
	}
	switch mode {
	case "conversation", "conversation-stop", "conversation-children", "conversation-children-unconfirmed":
		runtimetest.Conversation(mode)
	case "control", "reuse-control":
		controlHelper(mode == "reuse-control")
	case "malformed":
		write("private-input invalid json")
	case "missing-type":
		write(`{"private-input":"secret"}`)
	case "oversized":
		write(strings.Repeat("x", MaxFrameBytes+1))
	case "exit":
		os.Exit(3)
	case "flood":
		for range 100 {
			write(`{"type":"assistant"}`)
		}
	case "no-read":
		var first [1]byte
		if _, err := os.Stdin.Read(first[:]); err != nil {
			os.Exit(2)
		}
		write(`{"type":"system"}`)
		time.Sleep(time.Minute)
	case "echo":
		if _, err := fmt.Fprintln(os.Stderr, "private-input diagnostic"); err != nil {
			os.Exit(2)
		}
		write(`{"type":"system","session_id":"test-session"}`)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			write(scanner.Text())
		}
		if scanner.Err() != nil {
			os.Exit(2)
		}
	}
	os.Exit(0)
}
