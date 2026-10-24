package openai

import (
	"context"
	"strings"

	sdk "github.com/openai/openai-go/v3"

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

// capsFor returns what model takes. An unknown model is sent the effort
// asked for, and the API's answer says if it does not take it; the older
// gpt-4 family takes no reasoning at all.
func capsFor(model string) caps {
	if c, ok := knownCaps[provider.Undated(model)]; ok {
		return c
	}
	if strings.HasPrefix(model, "gpt-4") || strings.HasPrefix(model, "chatgpt-4") {
		return caps{}
	}
	return caps{reasoning: true}
}

// Models implements provider.Provider. The API lists every model the key
// can use, including images, audio, and embeddings; only text models that
// the Responses API serves are kept.
func (p *Provider) Models(ctx context.Context) ([]provider.Model, error) {
	opts, err := p.options()
	if err != nil {
		return nil, err
	}
	svc := sdk.NewModelService(opts...)
	pager := svc.ListAutoPaging(ctx)
	var out []provider.Model
	for pager.Next() {
		m := pager.Current()
		if !textModel(m.ID) {
			continue
		}
		out = append(out, provider.Model{ID: m.ID, DisplayName: m.ID})
	}
	if err := pager.Err(); err != nil {
		return nil, mapErr(ctx, err)
	}
	return out, nil
}

// notText marks model IDs for other kinds of output, or for APIs other
// than Responses.
var notText = []string{
	"audio", "realtime", "transcribe", "tts", "image", "search", "embedding",
	"moderation", "instruct", "dall-e", "whisper", "sora", "babbage", "davinci",
}

func textModel(id string) bool {
	if !strings.HasPrefix(id, "gpt-") && !strings.HasPrefix(id, "chatgpt-") &&
		!strings.HasPrefix(id, "codex-") && !isOSeries(id) {
		return false
	}
	for _, s := range notText {
		if strings.Contains(id, s) {
			return false
		}
	}
	return true
}

// isOSeries matches the o1, o3, and o4 reasoning models.
func isOSeries(id string) bool {
	return len(id) >= 2 && id[0] == 'o' && id[1] >= '1' && id[1] <= '9'
}
