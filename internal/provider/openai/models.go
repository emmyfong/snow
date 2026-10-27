package openai

import (
	"context"
	"strings"

	sdk "github.com/openai/openai-go/v3"

	"github.com/emmyfong/snow/internal/provider"
)

// Models implements provider.Provider. The API lists every model the key
// can use, including images, audio, and embeddings; only text models that
// the Responses API serves are kept. A compatible service offers the models
// from its config, or else its whole list.
func (p *Provider) Models(ctx context.Context) ([]provider.Model, error) {
	if len(p.models) > 0 {
		out := make([]provider.Model, len(p.models))
		for i, id := range p.models {
			out[i] = provider.Model{ID: id, DisplayName: id, Efforts: capsFor(id).supported()}
		}
		return out, nil
	}
	opts, key, err := p.options()
	if err != nil {
		return nil, err
	}
	svc := sdk.NewModelService(opts...)
	pager := svc.ListAutoPaging(ctx)
	var out []provider.Model
	for pager.Next() {
		m := pager.Current()
		if !p.compat && !textModel(m.ID) {
			continue
		}
		out = append(out, provider.Model{ID: m.ID, DisplayName: m.ID, Efforts: capsFor(m.ID).supported()})
	}
	if err := pager.Err(); err != nil {
		return nil, mapErr(ctx, err, key)
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
