package clauderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

// ModelPrefix is the provider part of a Claude Code chat's model name, as
// in "claude-code/opus".
const ModelPrefix = "claude-code/"

// DefaultModel is the model a new Claude Code chat starts with: Claude
// Code's own default.
const DefaultModel = ModelPrefix + "default"

// modelFlag returns what Claude Code's --model flag takes for model, or ""
// for Claude Code's own default.
func modelFlag(model string) string {
	alias := strings.TrimPrefix(model, ModelPrefix)
	if alias == "default" {
		return ""
	}
	return alias
}

// Models asks the selected Claude Code which models it offers, by alias
// such as "opus"; the chat names one as ModelPrefix plus the alias. It runs
// only Claude's initialize handshake, which needs no sign-in and sends no
// model request.
func (p Profile) Models(ctx context.Context, program shell.Profile, folder string) ([]api.ModelInfo, error) {
	launch, err := p.ChatLaunch(program, folder, "", RunOptions{})
	if err != nil {
		return nil, err
	}
	stream, err := StartStream(launch)
	if err != nil {
		return nil, err
	}
	client := NewClient(stream)
	result, err := client.Initialize(ctx)
	if stopErr := client.Stop(); stopErr != nil && !errors.Is(stopErr, ErrClosed) {
		err = errors.Join(err, stopErr)
	}
	if err != nil {
		return nil, err
	}
	return parseModels(result)
}

func parseModels(result json.RawMessage) ([]api.ModelInfo, error) {
	var reply struct {
		Models []struct {
			Value       string   `json:"value"`
			DisplayName string   `json:"displayName"`
			Efforts     []string `json:"supportedEffortLevels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(result, &reply); err != nil {
		return nil, fmt.Errorf("%w: read Claude Code's models: %w", ErrProtocol, err)
	}
	if len(reply.Models) == 0 {
		return nil, fmt.Errorf("%w: this Claude Code reports no models. Run claude update", ErrProtocol)
	}
	models := make([]api.ModelInfo, 0, len(reply.Models))
	for _, m := range reply.Models {
		if m.Value == "" || strings.ContainsAny(m.Value, "/ ") {
			continue
		}
		info := api.ModelInfo{ID: m.Value, DisplayName: m.DisplayName, Efforts: []api.EffortLevel{}}
		// Snow offers only the levels it knows, in its own order.
		for _, level := range api.EffortLevels {
			if slices.Contains(m.Efforts, level) {
				info.Efforts = append(info.Efforts, level)
			}
		}
		models = append(models, info)
	}
	return models, nil
}
