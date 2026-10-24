package providertest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// KeyServer is a fake vendor API for key checks. It accepts one key, sent
// as Anthropic's x-api-key header or OpenAI's bearer token, answers with an
// empty model list, and rejects any other key with 401. It counts the
// requests it gets.
type KeyServer struct {
	*httptest.Server
	requests atomic.Int32
}

// NewKeyServer starts a KeyServer that accepts key. The test closes it.
func NewKeyServer(t testing.TB, key string) *KeyServer {
	t.Helper()
	ks := &KeyServer{}
	ks.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ks.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Api-Key") != key && r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid key"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[],"has_more":false}`)
	}))
	t.Cleanup(ks.Close)
	return ks
}

// Requests returns how many requests reached the server.
func (ks *KeyServer) Requests() int { return int(ks.requests.Load()) }
