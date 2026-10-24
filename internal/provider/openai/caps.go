package openai

import (
	"strings"

	"github.com/emmyfong/snow/internal/provider"
)

// caps is what a model takes in a request. The model list does not say,
// so the values come from OpenAI's model pages.
type caps struct {
	reasoning bool
	efforts   map[provider.Effort]bool // nil: any level
}

func (c caps) takes(e provider.Effort) bool { return c.efforts == nil || c.efforts[e] }

var (
	toXHigh = efforts(provider.EffortLow, provider.EffortMedium, provider.EffortHigh, provider.EffortXHigh)
	toMax   = efforts(provider.EffortLow, provider.EffortMedium, provider.EffortHigh, provider.EffortXHigh, provider.EffortMax)
)

func efforts(es ...provider.Effort) map[provider.Effort]bool {
	m := map[provider.Effort]bool{}
	for _, e := range es {
		m[e] = true
	}
	return m
}

// knownCaps is by undated model ID, from
// https://developers.openai.com/api/docs/models on 2026-10-23.
var knownCaps = map[string]caps{
	"gpt-6-astra":   {reasoning: true, efforts: toMax},
	"gpt-6.1-sol":   {reasoning: true, efforts: toMax},
	"gpt-6-sol":     {reasoning: true, efforts: toMax},
	"gpt-6-luna":    {reasoning: true, efforts: toMax},
	"gpt-5.6-sol":   {reasoning: true, efforts: toMax},
	"gpt-5.6-terra": {reasoning: true, efforts: toMax},
	"gpt-5.6-luna":  {reasoning: true, efforts: toMax},
	"gpt-5.5":       {reasoning: true, efforts: toXHigh},
	"gpt-5.4":       {reasoning: true, efforts: toXHigh},
	"gpt-5.4-mini":  {reasoning: true, efforts: toXHigh},
	"gpt-5.4-nano":  {reasoning: true, efforts: toXHigh},
	"gpt-5.3-codex": {reasoning: true, efforts: toXHigh},
}

// capsFor returns what model takes. An unknown model of a reasoning
// family (o-series, gpt-5 and later, codex) is sent the effort asked for,
// and the API's answer says if it does not take it. Any other model, such
// as gpt-4.1, gpt-3.5, or a chat-latest alias, gets no reasoning at all.
func capsFor(model string) caps {
	if c, ok := knownCaps[provider.Undated(model)]; ok {
		return c
	}
	if reasoningFamily(model) {
		return caps{reasoning: true}
	}
	return caps{}
}

func reasoningFamily(id string) bool {
	if strings.Contains(id, "-chat") {
		return false
	}
	if isOSeries(id) || strings.HasPrefix(id, "codex-") {
		return true
	}
	rest, ok := strings.CutPrefix(id, "gpt-")
	if !ok {
		return false
	}
	major := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		major = major*10 + int(c-'0')
	}
	return major >= 5
}
