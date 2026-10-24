package provider

import (
	"bufio"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func anthropic(model string) ModelName { return ModelName{Provider: "anthropic", Model: model} }

func TestCost(t *testing.T) {
	tests := []struct {
		name  string
		model string
		u     Usage
		want  float64
	}{
		// 1M input at $4 plus 1M output at $20.
		{"input and output", "claude-opus-5-5", Usage{Input: 1_000_000, Output: 1_000_000}, 24},
		// 2,000 cache reads at $0.20/M = $0.0004; 500 cache writes at $5/M =
		// $0.0025; 3 input at $4/M = $0.000012; 7 output at $20/M = $0.00014.
		{"cache", "claude-opus-5-5", Usage{Input: 3, Output: 7, CacheRead: 2000, CacheWrite: 500}, 0.003052},
		// Haiku 5.5 up to 100,000 prompt tokens: 100,000 at $0.10/M.
		{"short prompt", "claude-haiku-5-5", Usage{Input: 100_000}, 0.01},
		// Over 100,000 the whole prompt moves to $0.50/M: 100,001 at $0.50/M,
		// and 1,000 output at $2.50/M.
		{"long prompt", "claude-haiku-5-5", Usage{Input: 100_001, Output: 1000}, 0.0500005 + 0.0025},
		{"no tokens", "claude-sonnet-5-5", Usage{}, 0},
		// The snapshot ID a stream reports prices as its alias.
		{"snapshot ID", "claude-opus-5-5-20261001", Usage{Input: 1_000_000}, 4},
	}
	for _, tt := range tests {
		got, ok := Cost(anthropic(tt.model), tt.u)
		if !ok || math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("%s: Cost = %v, %v; want %v", tt.name, got, ok, tt.want)
		}
	}
}

// TestCostUnknownModel: a model not in the list has no cost, never $0.
func TestCostUnknownModel(t *testing.T) {
	for _, n := range []ModelName{
		anthropic("claude-next"),
		anthropic("claude-next-20261001"),
		anthropic("claude-opus-5-5-2026"),
		anthropic("claude-opus-5-5-2026100x"),
		{Provider: "openai", Model: "claude-opus-5-5"},
	} {
		if got, ok := Cost(n, Usage{Input: 1000}); ok || got != 0 {
			t.Errorf("Cost(%s) = %v, %v; want unknown", n, got, ok)
		}
	}
}

func TestContextWindow(t *testing.T) {
	tests := []struct {
		model        ModelName
		fromProvider int
		want         int
	}{
		{anthropic("claude-opus-5-5"), 500_000, 500_000},
		{anthropic("claude-opus-5-5"), 0, 1_000_000},
		{anthropic("claude-haiku-4-5"), 0, 200_000},
		{anthropic("claude-haiku-4-5-20251001"), 0, 200_000},
		{anthropic("claude-next"), 0, 0},
		{anthropic("claude-next"), 300_000, 300_000},
	}
	for _, tt := range tests {
		if got := ContextWindow(tt.model, tt.fromProvider); got != tt.want {
			t.Errorf("ContextWindow(%s, %d) = %d, want %d", tt.model, tt.fromProvider, got, tt.want)
		}
	}
}

func TestListDate(t *testing.T) {
	for _, p := range []string{"anthropic", "openai"} {
		if _, err := time.Parse("2006-01-02", PriceListDate(p)); err != nil {
			t.Errorf("PriceListDate(%q) = %q, want YYYY-MM-DD", p, PriceListDate(p))
		}
	}
	if d := PriceListDate("unknown"); d != "" {
		t.Errorf("PriceListDate(unknown) = %q, want empty", d)
	}
}

// TestEveryModelPriced guards against a stale list: every model in each
// vendor's fixture needs an entry with prices and a context window.
func TestEveryModelPriced(t *testing.T) {
	for _, vendor := range []string{"anthropic", "openai"} {
		t.Run(vendor, func(t *testing.T) {
			f, err := os.Open("testdata/" + vendor + "_models.txt")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()
			sc := bufio.NewScanner(f)
			n := 0
			for sc.Scan() {
				id := strings.TrimSpace(sc.Text())
				if id == "" || strings.HasPrefix(id, "#") {
					continue
				}
				n++
				p, ok := LookupPrice(ModelName{Provider: vendor, Model: id})
				if !ok || p.Input <= 0 || p.Output <= 0 || p.CacheRead <= 0 || p.CacheWrite <= 0 || p.ContextWindow <= 0 {
					t.Errorf("%s has no complete price entry: %+v", id, p)
				}
			}
			if err := sc.Err(); err != nil || n == 0 {
				t.Fatalf("read the fixture: %v, %d models", err, n)
			}
		})
	}
}

// TestOpenAICost checks the long-context rates above 272K prompt tokens,
// counted with cached tokens.
func TestOpenAICost(t *testing.T) {
	m := ModelName{Provider: "openai", Model: "gpt-5.5-2026-04-23"}
	tests := []struct {
		u    Usage
		want float64
	}{
		{Usage{Input: 100_000}, 0.5},
		{Usage{Output: 1_000_000}, 30},
		{Usage{CacheRead: 100_000}, 0.05},
		{Usage{Input: 200_000, CacheRead: 72_000}, 0.2*5 + 0.072*0.50},
		{Usage{Input: 200_000, CacheRead: 72_001, Output: 1000}, 0.2*10 + 0.072001*1 + 0.001*45},
	}
	for _, tt := range tests {
		got, ok := Cost(m, tt.u)
		if !ok || math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("Cost(%+v) = %v, %v; want %v", tt.u, got, ok, tt.want)
		}
	}
}
