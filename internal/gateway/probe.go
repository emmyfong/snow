package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// probeTimeout bounds one question to a gateway. A starting gateway may not
// listen yet, so a failure returns at once rather than at this limit.
const probeTimeout = 3 * time.Second

// probeClient asks a local address. It never follows a redirect, because
// the answer only says whether something serves the port. A proxy must not
// stand between Snow and a gateway on this machine.
var probeClient = &http.Client{
	Timeout: probeTimeout,
	Transport: &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Probe asks the gateway at baseURL for its model list. An answer below 500
// means the address is up: a gateway may ask for a key, or not serve
// /models until it is ready. A 5xx says the program runs but is not ready.
func Probe(ctx context.Context, baseURL string) error {
	u := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("ask %s: %w", u, err)
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return fmt.Errorf("ask %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%s answered %s", u, resp.Status)
	}
	return nil
}
