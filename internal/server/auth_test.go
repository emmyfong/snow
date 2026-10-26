package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/signin"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// authKey is the key the fake provider accepts in the auth tests.
const authKey = "sk-good-SECRET"

// authOptions returns test options with an auth service that checks keys
// against a fake API and saves them in a temporary folder, that key folder,
// and the conversation folder.
func authOptions(t *testing.T) (Options, *providertest.KeyServer, string, string) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	srv := providertest.NewKeyServer(t, authKey)
	dir := t.TempDir()
	ps := signin.ProvidersAt(signin.BaseURLs{Anthropic: srv.URL, OpenAI: srv.URL})
	convDir := filepath.Join(t.TempDir(), "conversations")
	store, err := convstore.Open(convDir)
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	if err := reg.Register(providertest.New("fake")); err != nil {
		t.Fatal(err)
	}
	opts := testOptions()
	opts.Chat = &ChatOptions{
		Store: store, Providers: reg, Tools: tool.NewRegistry(),
		DefaultModel: chatModel,
		Auth:         signin.NewAuth(ps, dir),
	}
	return opts, srv, dir, convDir
}

// waitAuth waits for a message that ok accepts.
func waitAuth(t *testing.T, c *apitest.Client, what string, ok func(api.Message) bool) api.Message {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(100 * time.Millisecond)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatalf("waiting for %s: %v", what, err)
		}
		if m != nil && ok(m) {
			return m
		}
	}
	t.Fatalf("no %s", what)
	return nil
}

// TestAuthList: the list names every provider and says when a key is saved.
func TestAuthList(t *testing.T) {
	opts, _, _, _ := authOptions(t)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.AuthListRequest{})
	m := waitAuth(t, c, "the provider list", func(api.Message) bool { return true })
	list, ok := m.(*api.AuthList)
	if !ok || len(list.Providers) != 2 {
		t.Fatalf("first reply %#v, want an AuthList with two providers", m)
	}
	if p := list.Providers[0]; p.Name != "anthropic" || p.Title == "" || p.KeyURL == "" || p.Connected {
		t.Fatalf("anthropic %+v, want a disconnected provider with a key page", p)
	}
}

// TestLoginChecksKeyBeforeSave: a rejected key answers with an error, stays
// out of the store, and the next list still shows the provider without a key.
func TestLoginChecksKeyBeforeSave(t *testing.T) {
	opts, srv, dir, _ := authOptions(t)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.LoginRequest{Provider: "anthropic", Key: "sk-bad-SECRET"})
	m := waitAuth(t, c, "the rejected key", func(m api.Message) bool { return m.Type() == api.TypeError })
	e, ok := m.(*api.Error)
	if !ok || e.Code != api.CodeAuthFailed || !strings.Contains(e.Message, "did not accept the key") {
		t.Fatalf("reply %#v, want an auth failure that names the rejection", m)
	}
	if n := srv.Requests(); n != 1 {
		t.Fatalf("provider requests = %d, want the key checked once", n)
	}
	if _, _, err := credential.New(dir, nil).Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after a rejected key: %v, want no key", err)
	}

	c.Send(&api.AuthListRequest{})
	m = waitAuth(t, c, "the provider list after a rejection", func(m api.Message) bool {
		l, ok := m.(*api.AuthList)
		return ok && len(l.Providers) == 2
	})
	if l := m.(*api.AuthList); l.Providers[0].Connected {
		t.Fatalf("provider %+v is connected after a rejected key", l.Providers[0])
	}
}

// TestLogin: an accepted key is saved, the reply says where, and a fresh
// list reports the key.
func TestLogin(t *testing.T) {
	opts, _, dir, _ := authOptions(t)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.LoginRequest{Provider: "anthropic", Key: authKey})
	m := waitAuth(t, c, "the login result", func(m api.Message) bool { return m.Type() == api.TypeLogin })
	login, ok := m.(*api.Login)
	if !ok || login.Provider != "anthropic" || login.Source != "OS credential store" || login.Shadowed {
		t.Fatalf("reply %#v, want a saved key in the OS store", m)
	}
	m = waitAuth(t, c, "the fresh provider list", func(m api.Message) bool { return m.Type() == api.TypeAuthList })
	if p := m.(*api.AuthList).Providers[0]; !p.Connected || p.Source != "OS credential store" {
		t.Fatalf("provider %+v, want a connected key from the OS store", p)
	}
	k, src, err := credential.New(dir, nil).Get("anthropic")
	if err != nil || k.Reveal() != authKey || src != credential.SourceOS {
		t.Fatalf("saved key = %q from %v, error %v; want %q in the OS store", k, src, err, authKey)
	}
}

// TestLoginNetworkError: a check that cannot run saves nothing and says so.
func TestLoginNetworkError(t *testing.T) {
	opts, srv, dir, _ := authOptions(t)
	srv.Close()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.LoginRequest{Provider: "anthropic", Key: authKey})
	m := waitAuth(t, c, "the failed check", func(m api.Message) bool { return m.Type() == api.TypeError })
	e, ok := m.(*api.Error)
	if !ok || e.Code != api.CodeAuthFailed || !strings.Contains(e.Message, "could not check the key") {
		t.Fatalf("reply %#v, want a check failure", m)
	}
	if _, _, err := credential.New(dir, nil).Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after a failed check: %v, want no key", err)
	}
}

// TestLogout: the saved key is removed, and a fresh list says so.
func TestLogout(t *testing.T) {
	opts, _, dir, _ := authOptions(t)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)

	c.Send(&api.LoginRequest{Provider: "anthropic", Key: authKey})
	waitAuth(t, c, "the login result", func(m api.Message) bool { return m.Type() == api.TypeLogin })
	c.Send(&api.LogoutRequest{Provider: "anthropic"})
	m := waitAuth(t, c, "the logout result", func(m api.Message) bool { return m.Type() == api.TypeLogout })
	if out := m.(*api.Logout); out.Provider != "anthropic" {
		t.Fatalf("logout reply %+v", out)
	}
	m = waitAuth(t, c, "the fresh provider list", func(m api.Message) bool {
		l, ok := m.(*api.AuthList)
		return ok && len(l.Providers) == 2
	})
	if p := m.(*api.AuthList).Providers[0]; p.Connected || p.Source != "" {
		t.Fatalf("provider %+v is still connected after logout", p)
	}
	if _, _, err := credential.New(dir, nil).Get("anthropic"); !errors.Is(err, credential.ErrNoKey) {
		t.Fatalf("Get after logout: %v, want no key", err)
	}
}

// TestLogoutEnvironment: a key from the environment cannot be removed, and
// the error names the variable.
func TestLogoutEnvironment(t *testing.T) {
	opts, _, _, _ := authOptions(t)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)
	t.Setenv("ANTHROPIC_API_KEY", authKey)

	c.Send(&api.LogoutRequest{Provider: "anthropic"})
	m := waitAuth(t, c, "the refused logout", func(m api.Message) bool { return m.Type() == api.TypeError })
	e, ok := m.(*api.Error)
	if !ok || e.Code != api.CodeAuthFailed || !strings.Contains(e.Message, "ANTHROPIC_API_KEY") {
		t.Fatalf("reply %#v, want an environment-key failure", m)
	}
}

// TestAuthNotSetUp: a server without an auth service refuses auth requests.
func TestAuthNotSetUp(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.AuthListRequest{})
	m := waitAuth(t, c, "the refusal", func(m api.Message) bool { return m.Type() == api.TypeError })
	if e := m.(*api.Error); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, ErrNoAuth.Error()) {
		t.Fatalf("reply %+v, want ErrNoAuth", e)
	}
}

// lockedBuffer is a bytes.Buffer that goroutines may write at once.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// TestKeyNotLogged: a login key reaches the provider but never the log, a
// conversation file, or a client, whether the provider accepts it or not.
func TestKeyNotLogged(t *testing.T) {
	for _, tt := range []struct {
		name, key string
		reply     func(api.Message) bool
	}{
		{"accepted", authKey, func(m api.Message) bool { return m.Type() == api.TypeLogin }},
		{"rejected", "sk-ant-rejected-key-0123456789", func(m api.Message) bool {
			e, ok := m.(*api.Error)
			return ok && e.Code == api.CodeAuthFailed
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, srv, _, convDir := authOptions(t)
			logs := &lockedBuffer{}
			opts.Log = slog.New(slog.NewTextHandler(logs, nil))
			s := New(opts)
			t.Cleanup(s.Close)
			c := connect(t, s, 80, 24)
			openChat(t, c)

			c.Send(&api.LoginRequest{Provider: "anthropic", Key: tt.key})
			waitAuth(t, c, "the login reply", tt.reply)

			// Every message the client gets must be free of the key.
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				m, err := c.Next(50 * time.Millisecond)
				if errors.Is(err, apitest.ErrTimeout) {
					continue
				}
				if err != nil {
					break
				}
				b, err := json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(b, []byte(tt.key)) {
					t.Fatalf("a %T to the client holds the key: %s", m, b)
				}
			}
			if got := logs.String(); strings.Contains(got, tt.key) {
				t.Fatalf("the server log holds the key:\n%s", got)
			}
			files := 0
			if err := filepath.WalkDir(convDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				files++
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if bytes.Contains(b, []byte(tt.key)) {
					t.Fatalf("%s holds the key", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if files == 0 {
				t.Fatal("the chat wrote no conversation file")
			}
			if n := srv.Requests(); n != 1 {
				t.Fatalf("provider requests = %d, want the key checked once", n)
			}
		})
	}
}
