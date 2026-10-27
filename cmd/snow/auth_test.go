package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider/providertest"
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

// keyServer is the fake API of the running authTest.
var keyServer *providertest.KeyServer

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
	srv := providertest.NewKeyServer(t, goodKey)
	keyServer = srv
	old, oldIn := authProviders, stdin
	authProviders = func() ([]signin.Provider, error) {
		return signin.ProvidersAt(signin.BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL}), nil
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

// TestLoginBadInput: input that is not a key fails before any request
// and saves nothing.
func TestLoginBadInput(t *testing.T) {
	dir := authTest(t)
	tests := map[string]string{
		"":            "no input",
		"\n":          "one word with no spaces",
		"two words\n": "one word with no spaces",
	}
	for in, want := range tests {
		code, _, errOut := auth(t, in, "login", "openai")
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("input %q: code %d, err %q, want %q", in, code, errOut, want)
		}
	}
	if n := keyServer.Requests(); n != 0 {
		t.Fatalf("%d key checks went out for bad input", n)
	}
	if _, _, err := credential.New(dir, nil).Get("openai"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("bad input saved a key: %v", err)
	}
}

// TestLoginLineEndings: a key piped from Windows cmd.exe ends with
// " \r\n", and one from printf may have no line end at all.
func TestLoginLineEndings(t *testing.T) {
	for _, in := range []string{goodKey + " \r\n", goodKey} {
		dir := authTest(t)
		if code, _, errOut := auth(t, in, "login", "anthropic"); code != 0 {
			t.Fatalf("input %q: code %d, err %q", in, code, errOut)
		}
		if k, _, err := credential.New(dir, nil).Get("anthropic"); err != nil || k.Reveal() != goodKey {
			t.Fatalf("input %q: saved key differs: %v", in, err)
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
	if code, out, _ := auth(t, "y", "logout", "openai"); code != 0 || !strings.Contains(out, "Removed the openai key.") {
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
	if code != 1 || !strings.Contains(errOut, "environment (ANTHROPIC_API_KEY). Unset it") {
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
	if !strings.Contains(usage, "snow auth") || !strings.Contains(usage, "server, auth, and hook are commands") {
		t.Fatalf("usage does not list auth:\n%s", usage)
	}
}

// writeCredentials writes a credentials file with a line Snow cannot read
// as a key, as a hand edit can leave.
func writeCredentials(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestUnreadableKey: status says the key cannot be read, and logout still
// removes it.
func TestUnreadableKey(t *testing.T) {
	dir := authTest(t)
	writeCredentials(t, dir, "anthropic = two words\n")
	if _, out, _ := auth(t, "", "status"); !strings.Contains(out, "anthropic  cannot read the key:") {
		t.Fatalf("status: %q", out)
	}
	if code, out, errOut := auth(t, "y\n", "logout", "anthropic"); code != 0 || !strings.Contains(out, "Removed the anthropic key.") {
		t.Fatalf("logout: code %d, out %q, err %q", code, out, errOut)
	}
	if _, out, _ := auth(t, "", "status"); !strings.Contains(out, "anthropic  no key.") {
		t.Fatalf("status after logout: %q", out)
	}
}

// TestLoginCouldNotCheck: with the vendor unreachable, login saves nothing
// and does not blame the key.
func TestLoginCouldNotCheck(t *testing.T) {
	dir := authTest(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	authProviders = func() ([]signin.Provider, error) {
		return signin.ProvidersAt(signin.BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL}), nil
	}
	code, _, errOut := auth(t, goodKey+"\n", "login", "openai")
	if code != 1 || !strings.Contains(errOut, "could not check the key") || strings.Contains(errOut, "did not accept") {
		t.Fatalf("code %d, err %q", code, errOut)
	}
	if _, _, err := credential.New(dir, nil).Get("openai"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("saved without a check: %v", err)
	}
}
