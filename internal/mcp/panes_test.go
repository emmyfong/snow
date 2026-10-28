package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// fakeSnow is the Snow server end of a net.Pipe connection. It answers the
// handshake, sends the session list every connection gets, and answers the
// first request with answer.
func fakeSnow(t *testing.T, conn net.Conn, answer func(req api.Message) api.Message) {
	t.Helper()
	defer func() { _ = conn.Close() }()
	apiConn := api.NewConn(conn)
	if _, err := api.ServerHandshake(apiConn, api.Welcome{ServerVersion: "test", Protocol: api.ProtocolVersion}); err != nil {
		return
	}
	if err := apiConn.Send(&api.SessionList{Sessions: []api.SessionInfo{{Name: "work", State: api.StateRunning}}}); err != nil {
		return
	}
	req, err := apiConn.Receive()
	if err != nil {
		return
	}
	_ = apiConn.Send(answer(req))
}

// TestSnowList: a tool call skips the session list and returns the panes.
func TestSnowList(t *testing.T) {
	ours, theirs := net.Pipe()
	go fakeSnow(t, theirs, func(req api.Message) api.Message {
		r, ok := req.(*api.PanesRequest)
		if !ok {
			return &api.Error{Code: api.CodeBadCommand, Message: "not a panes request"}
		}
		return &api.Panes{Session: "work", Caller: r.Caller, Panes: []api.PaneSummary{{ID: 2, WindowName: "dev"}}}
	})
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	panes, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if panes.Session != "work" || panes.Caller != 7 || len(panes.Panes) != 1 || panes.Panes[0].ID != 2 {
		t.Fatalf("panes = %+v", panes)
	}
}

// TestSnowRead: a read_pane call sends the caller and returns the pane.
func TestSnowRead(t *testing.T) {
	ours, theirs := net.Pipe()
	go fakeSnow(t, theirs, func(req api.Message) api.Message {
		r, ok := req.(*api.ReadPaneRequest)
		if !ok {
			return &api.Error{Code: api.CodeBadCommand, Message: "not a read pane request"}
		}
		if r.Caller != 7 || r.Pane != 2 || r.Scrollback != 40 {
			return &api.Error{Code: api.CodeBadCommand, Message: fmt.Sprintf("request %+v", r)}
		}
		return &api.ReadPane{Pane: 2, Session: "work", Screen: []string{"$ "}}
	})
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	read, err := c.Read(context.Background(), 2, 40)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Pane != 2 || read.Session != "work" || len(read.Screen) != 1 {
		t.Fatalf("read = %+v", read)
	}
}

// TestSnowSend: a send_keys call sends the caller, the text, and the keys.
func TestSnowSend(t *testing.T) {
	keys := []api.Key{{Code: api.KeyEnter}, {Code: "c", Mod: []string{api.ModCtrl}}}
	ours, theirs := net.Pipe()
	go fakeSnow(t, theirs, func(req api.Message) api.Message {
		r, ok := req.(*api.SendKeysRequest)
		if !ok {
			return &api.Error{Code: api.CodeBadCommand, Message: "not a send keys request"}
		}
		if r.Caller != 7 || r.Pane != 2 || r.Text != "go test" || len(r.Keys) != 2 {
			return &api.Error{Code: api.CodeBadCommand, Message: fmt.Sprintf("request %+v", r)}
		}
		return &api.SendKeys{Pane: 2, Dropped: 7}
	})
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	sent, err := c.Send(context.Background(), 2, "go test", keys)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent.Pane != 2 || sent.Dropped != 7 {
		t.Fatalf("sent = %+v", sent)
	}
}

// TestSnowWait: a wait_for call sends the caller, the conditions, and the
// limits, and returns the result.
func TestSnowWait(t *testing.T) {
	ours, theirs := net.Pipe()
	go fakeSnow(t, theirs, func(req api.Message) api.Message {
		r, ok := req.(*api.WaitForRequest)
		if !ok {
			return &api.Error{Code: api.CodeBadCommand, Message: "not a wait request"}
		}
		if r.Caller != 7 || r.Pane != 2 || r.Text != "ok" || r.QuietMS != 0 || r.TimeoutMS != 1500 || !r.IncludeExisting {
			return &api.Error{Code: api.CodeBadCommand, Message: fmt.Sprintf("request %+v", r)}
		}
		return &api.WaitFor{Pane: 2, Result: api.WaitFound, Line: "ok"}
	})
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	got, err := c.Wait(context.Background(), 2, "ok", 0, 1500*time.Millisecond, true)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Pane != 2 || got.Result != api.WaitFound || got.Line != "ok" {
		t.Fatalf("wait = %+v", got)
	}
}

// TestSnowListError: a server Error reaches the tool as an error, with its
// code and message.
func TestSnowListError(t *testing.T) {
	ours, theirs := net.Pipe()
	go fakeSnow(t, theirs, func(api.Message) api.Message {
		return &api.Error{Code: api.CodeNoPane, Message: "your pane 7 is not in this Snow server"}
	})
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), api.CodeNoPane) {
		t.Fatalf("err = %v, want %s", err, api.CodeNoPane)
	}
}

// TestSnowListCancel: a cancelled call returns at once and leaves the
// connection closed.
func TestSnowListCancel(t *testing.T) {
	ours, theirs := net.Pipe()
	go func() {
		defer func() { _ = theirs.Close() }()
		apiConn := api.NewConn(theirs)
		if _, err := api.ServerHandshake(apiConn, api.Welcome{ServerVersion: "test", Protocol: api.ProtocolVersion}); err != nil {
			return
		}
		if _, err := apiConn.Receive(); err != nil {
			return
		}
		<-time.After(2 * time.Second) // no answer: the client cancels first
	}()
	c := &snow{caller: 7, dial: func() (net.Conn, error) { return ours, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.List(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a cancelled call took %v", took)
	}
}

// TestSnowListNoServer: no socket gives a clear error that wraps the
// transport's.
func TestSnowListNoServer(t *testing.T) {
	c := &snow{socket: t.TempDir() + "/none.sock", caller: 1}
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "reach Snow") {
		t.Fatalf("err = %v", err)
	}
}
