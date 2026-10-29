package openai

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
)

func TestToolSchema(t *testing.T) {
	got, err := toolSchema(provider.ToolDef{})
	if err != nil || !reflect.DeepEqual(got, map[string]any{"type": "object", "properties": map[string]any{}}) {
		t.Fatalf("no schema: %v, %v", got, err)
	}
	got, err = toolSchema(provider.ToolDef{Schema: json.RawMessage(`{"type":"object","required":["a"]}`)})
	if err != nil || got["required"] == nil {
		t.Fatalf("schema: %v, %v", got, err)
	}
	if _, err = toolSchema(provider.ToolDef{Schema: json.RawMessage(`{`)}); err == nil {
		t.Fatal("a broken schema gave no error")
	}
}

func TestToolOutput(t *testing.T) {
	for _, tt := range []struct {
		in   provider.ToolResult
		want string
	}{
		{provider.ToolResult{Content: "ok"}, "ok"},
		{provider.ToolResult{Content: "boom", IsError: true}, "Error: boom"},
	} {
		if got := toolOutput(&tt.in); got != tt.want {
			t.Errorf("toolOutput(%+v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOrEmptyObject(t *testing.T) {
	if got := orEmptyObject(""); got != "{}" {
		t.Errorf("empty gave %q", got)
	}
	if got := orEmptyObject(`{"a":1}`); got != `{"a":1}` {
		t.Errorf("arguments gave %q", got)
	}
}

// TestUsageOf: the input total holds the cached and cache-written tokens,
// and the result keeps them apart. A service that counts them oddly, with
// more cached than total, does not make a negative input.
func TestUsageOf(t *testing.T) {
	for _, tt := range []struct {
		in, out, cached, written int64
		want                     provider.Usage
	}{
		{12, 3, 4, 0, provider.Usage{Input: 8, Output: 3, CacheRead: 4}},
		{20, 5, 4, 6, provider.Usage{Input: 10, Output: 5, CacheRead: 4, CacheWrite: 6}},
		{3, 1, 9, 0, provider.Usage{Input: 0, Output: 1, CacheRead: 9}},
	} {
		if got := usageOf(tt.in, tt.out, tt.cached, tt.written); got != tt.want {
			t.Errorf("usageOf(%d, %d, %d, %d) = %+v, want %+v", tt.in, tt.out, tt.cached, tt.written, got, tt.want)
		}
	}
}
