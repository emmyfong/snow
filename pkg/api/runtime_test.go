package api

import (
	"fmt"
	"strings"
	"testing"
)

func TestRuntimeRequestHidesKey(t *testing.T) {
	request := RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeSaveKey, Connection: "selected", Key: "secret-runtime-key"}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if strings.Contains(fmt.Sprintf(format, request), request.Key) || strings.Contains(fmt.Sprintf(format, &request), request.Key) {
			t.Fatalf("format %s exposed a connection credential", format)
		}
	}
	for _, tc := range []struct {
		request RuntimeRequest
		valid   bool
	}{
		{request, true},
		{RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeCreate, Auth: "subscription"}, true},
		{RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeList}, true},
		{RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeCreate, Auth: "automatic"}, false},
		{RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeLogin, Key: "key", Connection: "selected"}, false},
		{RuntimeRequest{Pane: 1, Agent: "agent", Action: RuntimeUse}, false},
		{RuntimeRequest{Action: RuntimeList}, false},
	} {
		if err := tc.request.Validate(); (err == nil) != tc.valid {
			t.Errorf("action %s validation: %v", tc.request.Action, err)
		}
	}
}

func TestRuntimeSnapshotResync(t *testing.T) {
	snapshot := ChatSnapshot{Pane: 1, Agent: "agent"}
	info := &RuntimeInfo{Kind: "claude-code", Connection: "selected", Session: "backend", Tasks: []RuntimeTask{{ID: "child", Run: "run-1", State: "running"}}}
	snapshot.Apply(&ChatEvent{Pane: 1, Kind: ChatEventRuntime, Runtime: info})
	info.Tasks[0].State = "changed-later"
	if snapshot.Runtime == nil || snapshot.Runtime.Session != "backend" || snapshot.Runtime.Tasks[0].State != "running" {
		t.Fatal("runtime snapshot lost session identity or retained mutable task state")
	}
}

func TestRuntimeCostDoesNotDoubleCountRestoredSpend(t *testing.T) {
	snapshot := ChatSnapshot{Pane: 1}
	for _, cost := range []float64{0.1, 0.2, 0.25} {
		snapshot.Apply(&ChatEvent{Pane: 1, Kind: ChatEventUsage,
			Usage: &ChatUsage{Cost: cost, Priced: true, Cumulative: true, Scope: "main-agent"}})
	}
	if snapshot.Cost != 0.25 {
		t.Fatalf("summed cumulative runtime estimates: %v", snapshot.Cost)
	}
}
