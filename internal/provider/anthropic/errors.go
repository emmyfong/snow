package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/emmyfong/snow/internal/provider"
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
	if !errors.As(err, &apiErr) {
		return &provider.Error{Kind: provider.KindNetwork, Err: err}
	}
	errType, msg := apiMessage(apiErr)
	label := errType
	if label == "" {
		label = http.StatusText(apiErr.StatusCode)
	}
	cause := fmt.Errorf("HTTP %d %s: %s", apiErr.StatusCode, label, msg)
	pe := &provider.Error{Kind: kindOf(apiErr.StatusCode, errType, msg), Err: cause}
	if pe.Kind == provider.KindRateLimited && apiErr.Response != nil {
		pe.RetryAfter = retryAfter(apiErr.Response.Header.Get("retry-after"))
	}
	return pe
}

// kindOf reads the error type first, because an error inside a stream
// arrives with the stream's status, 200.
func kindOf(status int, errType, msg string) provider.Kind {
	switch errType {
	case "authentication_error", "permission_error":
		return provider.KindBadKey
	case "not_found_error":
		return provider.KindModelNotFound
	case "rate_limit_error":
		return provider.KindRateLimited
	case "overloaded_error":
		return provider.KindOverloaded
	case "request_too_large":
		return provider.KindContextTooLong
	case "invalid_request_error":
		if tooLong(msg) {
			return provider.KindContextTooLong
		}
		return provider.KindOther
	}
	switch status {
	case 401, 403:
		return provider.KindBadKey
	case 404:
		return provider.KindModelNotFound
	case 413:
		return provider.KindContextTooLong
	case 429:
		return provider.KindRateLimited
	case 502, 503, 504, 529:
		// A gateway or proxy in front of the API is busy or down.
		return provider.KindOverloaded
	}
	return provider.KindOther
}

func tooLong(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{"prompt is too long", "context window", "exceed context limit", "too many tokens"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// apiMessage reads the type and message from the API's error body:
// {"type":"error","error":{"type":"...","message":"..."}}.
func apiMessage(e *sdk.Error) (errType, msg string) {
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(e.RawJSON()), &body) == nil && body.Error.Type != "" {
		return body.Error.Type, cut(body.Error.Message)
	}
	return string(e.Type()), "no message"
}

// maxMessage keeps a vendor's message short enough for one line in the UI.
const maxMessage = 500

func cut(msg string) string {
	r := []rune(msg)
	if len(r) <= maxMessage {
		return msg
	}
	return string(r[:maxMessage]) + "..."
}

// maxRetryAfter caps a vendor's retry time, so a bad header cannot ask the
// user to wait for days.
const maxRetryAfter = time.Hour

// retryAfter reads a retry-after header in seconds. Zero means unknown.
func retryAfter(v string) time.Duration {
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return 0
	}
	if n >= maxRetryAfter.Seconds() {
		return maxRetryAfter
	}
	return time.Duration(n * float64(time.Second))
}
