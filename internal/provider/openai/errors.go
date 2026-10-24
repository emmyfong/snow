package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providerhttp"
)

// mapErr turns an SDK failure into a *provider.Error. The SDK's error holds
// the HTTP request, whose headers carry the key, so the cause is rebuilt
// from the status and the API's message alone. A canceled context passes
// through unchanged.
func mapErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		pe := failure(apiErr.StatusCode, apiErr.Code, apiErr.Type, apiErr.Message)
		if pe.Kind == provider.KindRateLimited && apiErr.Response != nil {
			pe.RetryAfter = retryAfter(apiErr.Response.Header)
		}
		return pe
	}
	var streamErr *ssestream.StreamError
	if errors.As(err, &streamErr) {
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(streamErr.Event.Data, &body)
		return failure(0, body.Error.Code, body.Error.Type, body.Error.Message)
	}
	return &provider.Error{Kind: provider.KindNetwork, Err: err}
}

// failure builds the error for an API refusal. status is 0 for an error
// inside a stream.
func failure(status int, code, errType, msg string) *provider.Error {
	label := code
	if label == "" {
		label = errType
	}
	if label == "" {
		label = http.StatusText(status)
	}
	if msg == "" {
		msg = "no message"
	}
	cause := fmt.Errorf("%s: %s", label, providerhttp.Cut(msg))
	if status != 0 {
		cause = fmt.Errorf("HTTP %d %w", status, cause)
	}
	return &provider.Error{Kind: kindOf(status, code, errType, msg), Err: cause}
}

// kindOf reads the error code first: an error inside a stream has no
// status, and a 429 can mean an empty balance, which a wait does not fix.
// A 403 is not a bad key: the key is valid but lacks a scope, a region,
// or access to the model, and the vendor's message says which.
func kindOf(status int, code, errType, msg string) provider.Kind {
	switch code {
	case "invalid_api_key", "invalid_authentication", "unauthorized":
		return provider.KindBadKey
	case "model_not_found":
		return provider.KindModelNotFound
	case "rate_limit_exceeded":
		return provider.KindRateLimited
	case "insufficient_quota":
		return provider.KindOther
	case "context_length_exceeded":
		return provider.KindContextTooLong
	case "server_is_overloaded", "slow_down":
		return provider.KindOverloaded
	}
	if errType == "insufficient_quota" {
		return provider.KindOther
	}
	if tooLong(msg) {
		return provider.KindContextTooLong
	}
	switch status {
	case 401:
		return provider.KindBadKey
	case 404:
		return provider.KindModelNotFound
	case 413:
		return provider.KindContextTooLong
	case 429:
		return provider.KindRateLimited
	case 502, 503, 504:
		// A gateway or proxy in front of the API is busy or down.
		return provider.KindOverloaded
	}
	return provider.KindOther
}

func tooLong(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{"maximum context length", "context window", "context_length_exceeded", "too many tokens"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// retryAfter reads OpenAI's retry-after-ms header, then the standard
// retry-after in seconds. Zero means unknown.
func retryAfter(h http.Header) time.Duration {
	if ms, err := strconv.ParseFloat(strings.TrimSpace(h.Get("retry-after-ms")), 64); err == nil {
		return providerhttp.Seconds(ms / 1000)
	}
	return providerhttp.RetryAfter(h.Get("retry-after"))
}
