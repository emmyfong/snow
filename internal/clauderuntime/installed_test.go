package clauderuntime

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// TestInstalledClaudeInitialize is an explicit, no-prompt compatibility probe.
// It creates an empty private profile, disables imported settings/MCP, and never
// sends login, status, logout or a user message. CI uses only helper fixtures.
func TestInstalledClaudeInitialize(t *testing.T) {
	executable := os.Getenv("SNOW_TEST_CLAUDE")
	if executable == "" {
		t.Skip("set SNOW_TEST_CLAUDE to probe an installed CLI without a model request")
	}
	profile, err := NewProfile(t.TempDir(), "compatibility-probe", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.Prepare(); err != nil {
		t.Fatal(err)
	}
	launch, err := profile.ChatLaunch(shell.Profile{Path: executable}, t.TempDir(), "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := StartStream(launch)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(stream)
	t.Cleanup(func() {
		if err := client.Stop(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := client.Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var capabilities map[string]json.RawMessage
	if json.Unmarshal(result, &capabilities) != nil || capabilities == nil {
		t.Fatal("initialization did not return a capability object")
	}
}
