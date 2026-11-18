package client

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

func TestRestartAskShowsHomeFirst(t *testing.T) {
	a := newRestartAskApp()
	a.askRestart(versionMismatch())
	if got := ansi.Strip(a.View().Content); !strings.Contains(got, "Restart it now?") {
		t.Fatalf("home page does not show the restart question:\n%s", got)
	}
	if a.result.Restart || a.result.Err != nil {
		t.Fatalf("opening the question changed the result: %+v", a.result)
	}
}

func TestRestartAskShowsSessionFirst(t *testing.T) {
	a := newRestartAskApp()
	openSession(t, a, "work")
	a.askRestart(versionMismatch())
	v := a.View()
	if v.WindowTitle != "snow: work" || !strings.Contains(ansi.Strip(v.Content), "Restart it now?") {
		t.Fatalf("first screen is not the session with the restart question: title %q\n%s", v.WindowTitle, ansi.Strip(v.Content))
	}
}

func TestRestartAskYesQuitsWithRestart(t *testing.T) {
	for _, tt := range []struct {
		name string
		act  func(*app) tea.Cmd
	}{
		{name: "y", act: func(a *app) tea.Cmd { _, cmd := a.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"})); return cmd }},
		{name: "click Restart", act: func(a *app) tea.Cmd { return clickRestart(t, a) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := newRestartAskApp()
			a.askRestart(versionMismatch())
			cmd := tt.act(a)
			if cmd == nil {
				t.Fatal("confirming Restart did not quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok || !a.result.Restart || a.result.Err != nil {
				t.Fatalf("result %+v, command %T; want restart quit", a.result, cmd())
			}
		})
	}
}

func TestRestartAskNoContinues(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  tea.Key
	}{
		{name: "n", key: tea.Key{Code: 'n', Text: "n"}},
		{name: "escape", key: tea.Key{Code: tea.KeyEscape}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := newRestartAskApp()
			a.askRestart(versionMismatch())
			_, cmd := a.Update(tea.KeyPressMsg(tt.key))
			if cmd != nil || a.restartPopup != nil || a.result.Restart || a.result.Err != nil {
				t.Fatalf("cancel result %+v, popup %v, command %v; want screen to continue", a.result, a.restartPopup != nil, cmd)
			}
		})
	}
}

func TestRestartAskCtrlCActsAsCancel(t *testing.T) {
	a := newRestartAskApp()
	a.askRestart(versionMismatch())
	_, cmd := a.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd != nil || a.restartPopup != nil || a.result.Restart || a.result.Err != nil {
		t.Fatalf("Ctrl+c result %+v, popup %v, command %v; want cancel", a.result, a.restartPopup != nil, cmd)
	}
}

func TestRestartAskProtocolMismatchNo(t *testing.T) {
	result, first := runWithProtocolMismatch(t, "n")
	var mismatch *api.VersionMismatchError
	if !errors.As(result.Err, &mismatch) || result.Restart {
		t.Fatalf("result %+v, want protocol mismatch error", result)
	}
	if first != nil {
		t.Fatalf("sent %T after a protocol mismatch; want no Attach", first)
	}
}

func TestRestartAskProtocolMismatchYes(t *testing.T) {
	result, first := runWithProtocolMismatch(t, "y")
	if result.Err != nil || !result.Restart {
		t.Fatalf("result %+v, want restart request", result)
	}
	if first != nil {
		t.Fatalf("sent %T before restart; want no Attach", first)
	}
}

func TestRestartAskGolden80x24(t *testing.T)  { restartAskGolden(t, 80, 24) }
func TestRestartAskGolden120x40(t *testing.T) { restartAskGolden(t, 120, 40) }

func restartAskGolden(t *testing.T, cols, rows int) {
	a := newRestartAskApp()
	a.cols, a.rows = cols, rows
	a.screen.SetSize(cols, rows)
	a.askRestart(versionMismatch())
	golden.RequireEqual(t, a.View().Content)
}

func newRestartAskApp() *app {
	a := newHomeApp(func(api.Message) {}, "v0.2.0", "/src", theme.Default())
	a.cols, a.rows = 80, 24
	a.screen.SetSize(80, 24)
	return a
}

func versionMismatch() *api.VersionMismatchError {
	return &api.VersionMismatchError{ClientVersion: "v0.2.0", ServerVersion: "v0.1.0", ClientProtocol: api.ProtocolVersion, ServerProtocol: api.ProtocolVersion}
}

func clickRestart(t *testing.T, a *app) tea.Cmd {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(a.View().Content), "\n") {
		if x := strings.Index(row, "[ Restart ]"); x >= 0 {
			_, cmd := a.Update(tea.MouseClickMsg{X: x + 2, Y: y, Button: tea.MouseLeft})
			return cmd
		}
	}
	t.Fatalf("Restart button not found:\n%s", ansi.Strip(a.View().Content))
	return nil
}

func runWithProtocolMismatch(t *testing.T, answer string) (Result, api.Message) {
	t.Helper()
	path := apitest.SocketPath(t)
	listener, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	first := make(chan api.Message, 1)
	serverDone := make(chan struct{})
	handshaken := make(chan struct{})
	go func() {
		defer close(serverDone)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = raw.Close() }()
		conn := api.NewConn(raw)
		_, _ = api.ServerHandshake(conn, api.Welcome{ServerVersion: "old", Protocol: api.ProtocolVersion - 1})
		close(handshaken)
		_ = raw.SetReadDeadline(time.Now().Add(2 * time.Second))
		msg, _ := conn.Receive()
		first <- msg
	}()

	input, keys := io.Pipe()
	defer func() { _ = input.Close(); _ = keys.Close() }()
	done := make(chan struct {
		result Result
		err    error
	}, 1)
	go func() {
		result, err := Run(Options{Version: "v0.2.0", Socket: path, Session: "work", Start: func() error { return nil }, Input: input, Output: io.Discard})
		done <- struct {
			result Result
			err    error
		}{result, err}
	}()
	select {
	case <-handshaken:
	case <-time.After(5 * time.Second):
		t.Fatal("client did not handshake")
	}
	if _, err := io.WriteString(keys, answer); err != nil {
		t.Fatalf("send answer: %v", err)
	}
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("Run: %v", got.err)
		}
		select {
		case msg := <-first:
			<-serverDone
			return got.result, msg
		case <-time.After(5 * time.Second):
			t.Fatal("server did not finish reading the client connection")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client did not answer the protocol question")
	}
	return Result{}, nil
}
