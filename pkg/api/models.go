package api

import (
	"encoding/json"
	"slices"
	"strings"
)

// ModelListRequest asks the server for the providers and the models they
// offer. Force asks for a fresh list, ignoring the server's cache. Pane,
// when it is a Claude Code chat, asks for the models that chat's Claude
// Code offers instead.
type ModelListRequest struct {
	Force bool `json:"force,omitempty"`
	Pane  int  `json:"pane,omitempty"`
}

// ModelList is the server's answer to ModelListRequest: one entry per
// provider, in the order the picker shows them. Pane is set when the list
// is for that Claude Code chat only.
type ModelList struct {
	Providers []ProviderModels `json:"providers"`
	Pane      int              `json:"pane,omitempty"`
}

// ProviderModels is one provider and its models.
type ProviderModels struct {
	Name      string `json:"name"` // the provider's part of a model name, for example "anthropic"
	Title     string `json:"title,omitempty"`
	Connected bool   `json:"connected,omitempty"` // whether a key is saved
	// Models is empty until the server fetched the list. A provider with
	// Err set may still show the models it knew before the error.
	Models []ModelInfo `json:"models"`
	// Err is why the list is missing or old, for example "no key".
	Err string `json:"err,omitempty"`
	// Refreshing says a fetch runs now; a later ModelList has the result.
	Refreshing bool `json:"refreshing,omitempty"`
	// Status is a gateway's state, for a [provider.<name>] table with
	// start: GatewayStarting, GatewayReady, or GatewayFailed. Empty for
	// other providers.
	Status string `json:"status,omitempty"`
}

// A gateway's state, as ProviderModels.Status reports it. A server skips
// the model fetch for a gateway that is starting or failed.
const (
	// GatewayStarting means Snow started the program and waits for the
	// address.
	GatewayStarting = "starting"
	// GatewayReady means the address answers.
	GatewayReady = "ready"
	// GatewayFailed means the program exited, or the address did not
	// answer in time. The log says why.
	GatewayFailed = "failed"
)

// ModelInfo is one model a provider offers.
type ModelInfo struct {
	ID            string `json:"id"`
	DisplayName   string `json:"displayName,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty"`
	// Efforts lists the reasoning effort levels the model takes, in
	// EffortLevels order. Empty means the model takes none.
	Efforts []EffortLevel `json:"efforts"`
}

// EffortLevel names one reasoning effort.
type EffortLevel = string

// EffortLevels lists every reasoning effort level, from the least to the
// most thinking.
var EffortLevels = [...]EffortLevel{"low", "medium", "high", "xhigh", "max"}

// EffortSupported reports whether model takes level.
func (m ModelInfo) EffortSupported(level EffortLevel) bool {
	return slices.Contains(m.Efforts, level)
}

// ParseModelName cuts "provider/model" into its parts and reports whether
// the name has that form. Whether the provider and model exist is for the
// server to find out.
func ParseModelName(s string) (provider, model string, ok bool) {
	provider, model, ok = strings.Cut(s, "/")
	return provider, model, ok && provider != "" && model != ""
}

// MarshalJSON writes Models and Efforts as [] when they are empty.
func (l ModelList) MarshalJSON() ([]byte, error) {
	type wire ModelList
	if l.Providers == nil {
		l.Providers = []ProviderModels{}
	}
	for i := range l.Providers {
		if l.Providers[i].Models == nil {
			l.Providers[i].Models = []ModelInfo{}
		}
	}
	return json.Marshal(wire(l))
}

// Type implements Message.
func (*ModelListRequest) Type() MsgType { return TypeModelListRequest }
func (*ModelListRequest) message()      {}

// Type implements Message.
func (*ModelList) Type() MsgType { return TypeModelList }
func (*ModelList) message()      {}

// MarshalJSON writes Efforts as [] when it is empty.
func (m ModelInfo) MarshalJSON() ([]byte, error) {
	type wire ModelInfo
	if m.Efforts == nil {
		m.Efforts = []EffortLevel{}
	}
	return json.Marshal(wire(m))
}
