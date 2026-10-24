package providerhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	tests := map[string]time.Duration{
		"30":                            30 * time.Second,
		" 1.5 ":                         1500 * time.Millisecond,
		"":                              0,
		"0":                             0,
		"-5":                            0,
		"NaN":                           0,
		"Inf":                           0,
		"-Inf":                          0,
		"1e30":                          time.Hour,
		"7200":                          time.Hour,
		"Wed, 21 Oct 2026 07:28:00 GMT": 0,
	}
	for in, want := range tests {
		if got := RetryAfter(in); got != want {
			t.Errorf("RetryAfter(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCut(t *testing.T) {
	if got := Cut("short"); got != "short" {
		t.Errorf("Cut(short) = %q", got)
	}
	got := Cut(strings.Repeat("é", 2000))
	if len([]rune(got)) != MaxMessage+3 || !strings.HasSuffix(got, "...") {
		t.Errorf("Cut kept %d runes: %q...", len([]rune(got)), got[:10])
	}
}

// TestGuardedRedirect: a same-host redirect is followed; one to another
// host is refused before the target sees the header.
func TestGuardedRedirect(t *testing.T) {
	var mu sync.Mutex
	var leaked bool
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		leaked = leaked || r.Header.Get("X-Key") != ""
		mu.Unlock()
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/same" {
			http.Redirect(w, r, "/done", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/done" {
			_, _ = io.WriteString(w, "ok")
			return
		}
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	c := Guarded(nil)
	get := func(path string) error {
		req, err := http.NewRequest(http.MethodGet, origin.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Key", "secret")
		resp, err := c.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}
	if err := get("/same"); err != nil {
		t.Fatalf("same-host redirect: %v", err)
	}
	if err := get("/other"); err == nil || !strings.Contains(err.Error(), "another host") {
		t.Fatalf("cross-host redirect: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if leaked {
		t.Fatal("the other host received the header")
	}
}

// TestGuardedKeepsClient: Guarded copies the given client and leaves the
// original unchanged.
func TestGuardedKeepsClient(t *testing.T) {
	orig := &http.Client{Timeout: time.Second}
	g := Guarded(orig)
	if g == orig || g.Timeout != time.Second || orig.CheckRedirect != nil || g.CheckRedirect == nil {
		t.Fatalf("Guarded did not copy: %+v", g)
	}
}
