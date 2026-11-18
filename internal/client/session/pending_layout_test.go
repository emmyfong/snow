package session

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestPendingLayoutAppliesOnceToFirstPane(t *testing.T) {
	var sent []api.Message
	m := New(func(msg api.Message) { sent = append(sent, msg) })
	m.SetPendingLayout("focus")
	layout := &api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Panes: []api.PaneInfo{{ID: 3}}},
		{Index: 2, Panes: []api.PaneInfo{{ID: 9}, {ID: 10}}},
	}}
	m.Update(layout)
	if len(sent) != 1 {
		t.Fatalf("first layout sent %d messages, want one", len(sent))
	}
	cmd, ok := sent[0].(*api.Command)
	if !ok || cmd.Op != api.OpLayout || cmd.Target != 9 || cmd.Args[api.ArgName] != "focus" {
		t.Fatalf("first message = %#v, want layout focus for pane 9", sent[0])
	}
	m.Update(layout)
	if len(sent) != 1 {
		t.Fatalf("second layout sent more messages: %#v", sent[1:])
	}
}

func TestPendingLayoutWaitsForFirstPane(t *testing.T) {
	var sent []api.Message
	m := New(func(msg api.Message) { sent = append(sent, msg) })
	m.SetPendingLayout("ide")
	m.Update(&api.Layout{Session: "api", Active: 1})
	if len(sent) != 0 {
		t.Fatalf("empty layout sent messages: %#v", sent)
	}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 4}}}}})
	if len(sent) != 1 {
		t.Fatalf("layout with pane sent %d messages, want one", len(sent))
	}
}
