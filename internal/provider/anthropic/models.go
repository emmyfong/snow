package anthropic

import (
	"context"
	"slices"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/emmyfong/snow/internal/provider"
)

// caps is what a model takes in a request.
type caps struct {
	adaptive bool
	efforts  map[provider.Effort]bool // nil: any level
}

// unknownCaps is for a model the API did not describe: current models take
// adaptive thinking and every effort level.
var unknownCaps = caps{adaptive: true}

func (c caps) takes(e provider.Effort) bool { return c.efforts == nil || c.efforts[e] }

// supported lists the effort levels the model takes, in AllEfforts order.
func (c caps) supported() []provider.Effort {
	if c.efforts == nil {
		return slices.Clone(provider.AllEfforts[:])
	}
	var out []provider.Effort
	for _, e := range provider.AllEfforts {
		if c.efforts[e] {
			out = append(out, e)
		}
	}
	return out
}

// Models implements provider.Provider. Retired models are left out. It
// also records what each model takes, for Stream.
func (p *Provider) Models(ctx context.Context) ([]provider.Model, error) {
	c, err := p.sdkClient()
	if err != nil {
		return nil, err
	}
	var out []provider.Model
	found := map[string]caps{}
	pager := c.Models.ListAutoPaging(ctx, sdk.ModelListParams{})
	for pager.Next() {
		m := pager.Current()
		if m.Lifecycle == sdk.ModelInfoLifecycleRetired {
			continue
		}
		mc := capsOf(m.Capabilities)
		out = append(out, provider.Model{ID: m.ID, DisplayName: m.DisplayName, ContextWindow: int(m.MaxInputTokens), Efforts: mc.supported()})
		found[m.ID] = mc
		// A request may name the model without its snapshot date.
		if base := provider.Undated(m.ID); base != m.ID {
			if _, ok := found[base]; !ok {
				found[base] = mc
			}
		}
	}
	if err := pager.Err(); err != nil {
		return nil, mapErr(ctx, err)
	}
	p.mu.Lock()
	p.caps, p.listed = found, true
	p.mu.Unlock()
	return out, nil
}

// capsOf reads a model's capabilities. A response without them describes
// nothing, so the model counts as unknown.
func capsOf(c sdk.ModelCapabilities) caps {
	if !c.JSON.Thinking.Valid() || !c.JSON.Effort.Valid() {
		return unknownCaps
	}
	mc := caps{adaptive: c.Thinking.Supported && c.Thinking.Types.Adaptive.Supported, efforts: map[provider.Effort]bool{}}
	if c.Effort.Supported {
		mc.efforts[provider.EffortLow] = c.Effort.Low.Supported
		mc.efforts[provider.EffortMedium] = c.Effort.Medium.Supported
		mc.efforts[provider.EffortHigh] = c.Effort.High.Supported
		mc.efforts[provider.EffortXHigh] = c.Effort.Xhigh.Supported
		mc.efforts[provider.EffortMax] = c.Effort.Max.Supported
	}
	return mc
}

// capsFor returns what model takes, listing the models once if needed. A
// failed list does not stop the turn: the request goes out as for an
// unknown model, and the API's answer says what is wrong.
func (p *Provider) capsFor(ctx context.Context, model string) (caps, error) {
	p.mu.Lock()
	mc, ok := p.caps[model]
	listed := p.listed
	p.mu.Unlock()
	if ok {
		return mc, nil
	}
	if !listed {
		if _, err := p.Models(ctx); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return caps{}, ctxErr
			}
			return unknownCaps, nil
		}
		p.mu.Lock()
		mc, ok = p.caps[model]
		p.mu.Unlock()
		if ok {
			return mc, nil
		}
	}
	return unknownCaps, nil
}
