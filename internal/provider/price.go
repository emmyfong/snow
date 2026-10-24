package provider

// priceListDates are the days each vendor's built-in prices were copied
// from its pricing page. The list changes only with a Snow release.
var priceListDates = map[string]string{
	"anthropic": "2026-10-21",
	"openai":    "2026-10-23",
}

// PriceListDate returns the day the built-in prices of a provider were
// copied, as YYYY-MM-DD, or "" when Snow has no prices for it.
func PriceListDate(provider string) string { return priceListDates[provider] }

// Price is what a model costs in US dollars per million tokens, and its
// context window in tokens.
type Price struct {
	Input         float64
	Output        float64
	CacheRead     float64
	CacheWrite    float64 // Anthropic's 5-minute cache, which Snow uses
	ContextWindow int

	// Long, when set, replaces the rates for a prompt of more than
	// LongAbove tokens.
	Long      *Price
	LongAbove int
}

// prices are the built-in entries, from
// https://platform.claude.com/docs/en/about-claude/pricing and
// https://developers.openai.com/api/docs/pricing (Standard tier). OpenAI
// bills a cache write at the input rate.
var prices = map[ModelName]Price{
	{"anthropic", "claude-fable-5-1"}:  {Input: 10, Output: 50, CacheRead: 0.25, CacheWrite: 12.50, ContextWindow: 1_000_000},
	{"anthropic", "claude-fable-5"}:    {Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12.50, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-5-5"}:   {Input: 4, Output: 20, CacheRead: 0.20, CacheWrite: 5, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-5"}:     {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite: 6.25, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-4-8"}:   {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite: 6.25, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-4-7"}:   {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite: 6.25, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-4-6"}:   {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite: 6.25, ContextWindow: 1_000_000},
	{"anthropic", "claude-opus-4-5"}:   {Input: 5, Output: 25, CacheRead: 0.50, CacheWrite: 6.25, ContextWindow: 200_000},
	{"anthropic", "claude-sonnet-5-5"}: {Input: 2, Output: 10, CacheRead: 0.10, CacheWrite: 2.50, ContextWindow: 1_000_000},
	{"anthropic", "claude-sonnet-5"}:   {Input: 2, Output: 10, CacheRead: 0.20, CacheWrite: 2.50, ContextWindow: 1_000_000},
	{"anthropic", "claude-sonnet-4-6"}: {Input: 3, Output: 15, CacheRead: 0.30, CacheWrite: 3.75, ContextWindow: 1_000_000},
	{"anthropic", "claude-sonnet-4-5"}: {Input: 3, Output: 15, CacheRead: 0.30, CacheWrite: 3.75, ContextWindow: 200_000},
	{"anthropic", "claude-haiku-4-5"}:  {Input: 1, Output: 5, CacheRead: 0.10, CacheWrite: 1.25, ContextWindow: 200_000},
	{"anthropic", "claude-haiku-5-5"}: {
		Input: 0.10, Output: 0.50, CacheRead: 0.01, CacheWrite: 0.125, ContextWindow: 1_000_000,
		LongAbove: 100_000, Long: &Price{Input: 0.50, Output: 2.50, CacheRead: 0.05, CacheWrite: 0.625},
	},
	{"openai", "gpt-6-astra"}: {
		Input: 10, Output: 50, CacheRead: 1, CacheWrite: 10, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 20, Output: 75, CacheRead: 2, CacheWrite: 20},
	},
	{"openai", "gpt-6.1-sol"}: {
		Input: 2, Output: 10, CacheRead: 0.10, CacheWrite: 2, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 4, Output: 15, CacheRead: 0.20, CacheWrite: 4},
	},
	{"openai", "gpt-6-sol"}: {
		Input: 2, Output: 10, CacheRead: 0.20, CacheWrite: 2, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 4, Output: 15, CacheRead: 0.40, CacheWrite: 4},
	},
	{"openai", "gpt-6-luna"}: {
		Input: 0.10, Output: 0.50, CacheRead: 0.01, CacheWrite: 0.10, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 0.20, Output: 0.75, CacheRead: 0.02, CacheWrite: 0.20},
	},
	{"openai", "gpt-5.6-sol"}: {
		Input: 4, Output: 20, CacheRead: 0.40, CacheWrite: 4, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 8, Output: 30, CacheRead: 0.80, CacheWrite: 8},
	},
	{"openai", "gpt-5.6-terra"}: {
		Input: 2, Output: 12, CacheRead: 0.20, CacheWrite: 2, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 4, Output: 18, CacheRead: 0.40, CacheWrite: 4},
	},
	{"openai", "gpt-5.6-luna"}: {
		Input: 0.20, Output: 1.20, CacheRead: 0.02, CacheWrite: 0.20, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 0.40, Output: 1.80, CacheRead: 0.04, CacheWrite: 0.40},
	},
	{"openai", "gpt-5.5"}: {
		Input: 5, Output: 30, CacheRead: 0.50, CacheWrite: 5, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 10, Output: 45, CacheRead: 1, CacheWrite: 10},
	},
	{"openai", "gpt-5.4"}: {
		Input: 2.50, Output: 15, CacheRead: 0.25, CacheWrite: 2.50, ContextWindow: 1_050_000,
		LongAbove: openAILongAbove, Long: &Price{Input: 5, Output: 22.50, CacheRead: 0.50, CacheWrite: 5},
	},
	{"openai", "gpt-5.4-mini"}:  {Input: 0.75, Output: 4.50, CacheRead: 0.075, CacheWrite: 0.75, ContextWindow: 400_000},
	{"openai", "gpt-5.4-nano"}:  {Input: 0.20, Output: 1.25, CacheRead: 0.02, CacheWrite: 0.20, ContextWindow: 400_000},
	{"openai", "gpt-5.3-codex"}: {Input: 1.75, Output: 14, CacheRead: 0.175, CacheWrite: 1.75, ContextWindow: 400_000},
}

// openAILongAbove is the prompt size above which OpenAI's long-context
// rates apply.
const openAILongAbove = 272_000

// LookupPrice returns the built-in price of a model. A snapshot ID with a
// date, such as claude-opus-5-5-20261001 or gpt-5.5-2026-04-23, also
// matches the entry without the date, because a stream reports the
// snapshot it used.
func LookupPrice(name ModelName) (Price, bool) {
	if p, ok := prices[name]; ok {
		return p, true
	}
	if base := Undated(name.Model); base != name.Model {
		p, ok := prices[ModelName{Provider: name.Provider, Model: base}]
		return p, ok
	}
	return Price{}, false
}

// Cost estimates what usage cost in US dollars. u is the usage of one
// request: the long-prompt rates depend on that request's prompt size, so
// add up the costs of requests, not their usage. ok is false when the
// model is not in the list: the cost is unknown, not zero. Callers label
// the number as an estimate.
func Cost(name ModelName, u Usage) (dollars float64, ok bool) {
	p, ok := LookupPrice(name)
	if !ok {
		return 0, false
	}
	rates := p
	if p.Long != nil && u.Input+u.CacheRead+u.CacheWrite > p.LongAbove {
		rates = *p.Long
	}
	const perToken = 1.0 / 1_000_000
	return (float64(u.Input)*rates.Input +
		float64(u.Output)*rates.Output +
		float64(u.CacheRead)*rates.CacheRead +
		float64(u.CacheWrite)*rates.CacheWrite) * perToken, true
}

// ContextWindow returns a model's context window in tokens: fromProvider
// when the provider's model list gave one (non-zero), else the built-in
// value, else 0 for unknown.
func ContextWindow(name ModelName, fromProvider int) int {
	if fromProvider > 0 {
		return fromProvider
	}
	p, _ := LookupPrice(name)
	return p.ContextWindow
}
