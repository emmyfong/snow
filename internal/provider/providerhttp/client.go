// Package providerhttp holds the HTTP rules every provider shares: a client
// that keeps a key on its host, a reader for retry-after headers, and a cut
// for vendor messages that would not fit one line.
package providerhttp

import (
	"errors"
	"net/http"
	"time"
)

// Guarded copies c, or builds a client like the vendor SDKs' default, and
// makes it refuse a redirect to another host: Go's client would send the
// key header along.
func Guarded(c *http.Client) *http.Client {
	var g http.Client
	if c != nil {
		g = *c
	} else if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t = t.Clone()
		t.ResponseHeaderTimeout = 10 * time.Minute
		g.Transport = t
	}
	g.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != via[0].URL.Host {
			return errors.New("refused a redirect to another host")
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &g
}
