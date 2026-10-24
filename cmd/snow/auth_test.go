package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/signin"
)

const (
	goodKey = "sk-good-SECRET"
	badKey  = "sk-bad-SECRET"
)

func TestParseAuthArgs(t *testing.T) {
	tests := []struct {
		args    []string
		want    authCommand
		problem string // part of the error; "" means none
	}{
		{[]string{"status"}, authCommand{action: authStatus}, ""},
		{[]string{"login", "anthropic"}, authCommand{action: authLogin, provider: "anthropic"}, ""},
		{[]string{"login", "openai", "--api-key"}, authCommand{action: authLogin, provider: "openai"}, ""},
		{[]string{"login", "--api-key", "openai"}, authCommand{action: authLogin, provider: "openai"}, ""},
		{[]string{"logout", "openai"}, authCommand{action: authLogout, provider: "openai"}, ""},
		{nil, authCommand{}, "needs login, logout, or status"},
		{[]string{"login"}, authCommand{}, "needs one provider"},
		{[]string{"login", "a", "b"}, authCommand{}, "needs one provider"},
		{[]string{"login", "gemini"}, authCommand{}, "unknown provider"},
		{[]string{"logout", "openai", "--api-key"}, authCommand{}, "unknown option --api-key"},
		{[]string{"status", "openai"}, authCommand{}, "takes no provider"},
		{[]string{"whoami"}, authCommand{}, "unknown auth command"},
	}
	for _, tt := range tests {
		got, err := parseAuth(tt.args)
		if got != tt.want || (err == nil) != (tt.problem == "") || (err != nil && !strings.Contains(err.Error(), tt.problem)) {
			t.Errorf("parseAuth(%q) = %+v, %v; want %+v, %q", tt.args, got, err, tt.want, tt.problem)
		}
	}
}

// authTest sets up `snow auth` with a temporary home folder, an in-memory
// OS store, no keys in the environment, and key checks against a fake API
// that accepts only goodKey. It returns the folder of the credentials file.
func authTest(t *testing.T) string {
	t.Helper()
	keyring.MockInit()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Api-Key") != goodKey && r.Header.Get("Authorization") != "Bearer "+goodKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid key"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[],"has_more":false}`)
	}))
	t.Cleanup(srv.Close)
	old, oldIn := authProviders, stdin
	authProviders = func() []signin.Provider {
		return signin.ProvidersAt(signin.BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL})
	}
	t.Cleanup(func() { authProviders, stdin = old, oldIn })
	return filepath.Join(home, ".snow")
}

// auth runs `snow auth args...` with input on stdin.
func auth(t *testing.T, input string, args ...string) (code int, out, errOut string) {
	t.Helper()
	stdin = strings.NewReader(input)
	var o, e bytes.Buffer
	code = run(append([]string{"auth"}, args...), &o, &e)
	for _, s := range []string{o.String(), e.String()} {
		if strings.Contains(s, goodKey) || strings.Contains(s, badKey) {
			t.Fatalf("output holds a key: %q", s)
		}
	}
	return code, o.String(), e.String()
}

func TestLoginAPIKey(t *testing.T) {
	dir := authTest(t)
	code, out, errOut := auth(t, goodKey+"\n", "login", "anthropic")
	if code != 0 || !strings.Contains(out, "platform.claude.com") || !strings.Contains(out, "Saved the anthropic key in the OS credential store.") {
		t.Fatalf("login: code %d, out %q, err %q", code, out, errOut)
	}
	k, src, err := credential.New(dir, nil).Get("anthropic")
	if err != nil || src != credential.SourceOS || k.Reveal() != goodKey {
		t.Fatalf("saved key: %v from %v", err, src)
	}

	code, _, errOut = auth(t, badKey+"\n", "login", "openai")
	if code != 1 || !strings.Contains(errOut, "did not accept the key") {
		t.Fatalf("bad key: code %d, err %q", code, errOut)
	}
	if _, _, err := credential.New(dir, nil).Get("openai"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("a rejected key was saved: %v", err)
	}
}

func TestLoginNoInput(t *testing.T) {
	authTest(t)
	for _, in := range []string{"", "\n", "two words\n"} {
		if code, _, errOut := auth(t, in, "login", "openai"); code != 1 || errOut == "" {
			t.Errorf("input %q: code %d, err %q", in, code, errOut)
		}
	}
}

// TestLoginEnvironmentWins: a saved key still works with the environment
// variable set, and login says the variable takes precedence.
func TestLoginEnvironmentWins(t *testing.T) {
	authTest(t)
	t.Setenv("OPENAI_API_KEY", "sk-env")
	code, out, _ := auth(t, goodKey+"\n", "login", "openai")
	if code != 0 || !strings.Contains(out, "OPENAI_API_KEY is set") {
		t.Fatalf("code %d, out %q", code, out)
	}
}

func TestLogoutConfirm(t *testing.T) {
	dir := authTest(t)
	if code, _, errOut := auth(t, goodKey+"\n", "login", "openai"); code != 0 {
		t.Fatal(errOut)
	}
	if code, out, _ := auth(t, "n\n", "logout", "openai"); code != 0 || !strings.Contains(out, "Kept the key.") {
		t.Fatalf("answer n: code %d, out %q", code, out)
	}
	if _, _, err := credential.New(dir, nil).Get("openai"); err != nil {
		t.Fatalf("the key went after n: %v", err)
	}
	if code, out, _ := auth(t, "y\n", "logout", "openai"); code != 0 || !strings.Contains(out, "Removed the openai key.") {
		t.Fatalf("answer y: code %d, out %q", code, out)
	}
	if _, _, err := credential.New(dir, nil).Get("openai"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("the key stayed after y: %v", err)
	}
	if code, out, _ := auth(t, "", "logout", "openai"); code != 0 || !strings.Contains(out, "No openai key is saved.") {
		t.Fatalf("nothing saved: code %d, out %q", code, out)
	}
}

func TestLogoutEnvironment(t *testing.T) {
	authTest(t)
	t.Setenv("ANTHROPIC_API_KEY", goodKey)
	code, _, errOut := auth(t, "y\n", "logout", "anthropic")
	if code != 1 || !strings.Contains(errOut, "comes from ANTHROPIC_API_KEY") {
		t.Fatalf("code %d, err %q", code, errOut)
	}
}

func TestStatusHidesKeys(t *testing.T) {
	authTest(t)
	if code, _, errOut := auth(t, goodKey+"\n", "login", "anthropic"); code != 0 {
		t.Fatal(errOut)
	}
	t.Setenv("OPENAI_API_KEY", badKey)
	code, out, _ := auth(t, "", "status")
	want := "anthropic  API key (OS credential store)\nopenai     API key (environment)\n"
	if code != 0 || out != want {
		t.Fatalf("status: code %d, out %q, want %q", code, out, want)
	}
	t.Setenv("OPENAI_API_KEY", "")
	if _, out, _ = auth(t, "", "status"); !strings.Contains(out, "openai     no key. Run: snow auth login openai") {
		t.Fatalf("status without a key: %q", out)
	}
}

// TestSessionNamedAuth: "auth" is a command, never a session name.
func TestSessionNamedAuth(t *testing.T) {
	if got, err := parseArgs([]string{"auth"}); err == nil || got.kind == kindAttach {
		t.Fatalf("parseArgs(auth) = %+v, %v; want an auth error", got, err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"auth"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "snow auth login <provider>") {
		t.Fatalf("snow auth: code %d, err %q", code, errOut.String())
	}
	out.Reset()
	if code := run([]string{"auth", "help"}, &out, &errOut); code != 0 || out.String() != authUsage {
		t.Fatalf("snow auth help: code %d, out %q", code, out.String())
	}
	if !strings.Contains(usage, "snow auth") || !strings.Contains(usage, "server, and auth are commands") {
		t.Fatalf("usage does not list auth:\n%s", usage)
	}
}
