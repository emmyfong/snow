package signin

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/pkg/api"
)

// Auth lists the providers, checks and saves their keys, and removes them.
// It is safe for use from several goroutines: the server runs its auth calls
// off the state goroutine.
type Auth struct {
	providers []Provider
	store     *credential.Store

	mu     sync.Mutex
	notice string
}

// NewAuth returns the auth service for ps, reading and saving keys in dir
// (for example ~/.snow). The credential store's one-time messages, such as
// the warning that keys go to the fallback file, reach the next Login
// result.
func NewAuth(ps []Provider, dir string) *Auth {
	a := &Auth{providers: ps}
	a.store = credential.New(dir, a.Notice)
	return a
}

// Keys returns the key lookup the service uses, for providers that read
// keys through provider.KeySource.
func (a *Auth) Keys() *credential.Store { return a.store }

// Providers reports every provider and the state of its key, in picker
// order.
func (a *Auth) Providers() []api.ProviderAuth {
	statuses := Statuses(a.providers, a.store)
	out := make([]api.ProviderAuth, len(statuses))
	for i, s := range statuses {
		pa := api.ProviderAuth{
			Name:      s.Provider.Name,
			Title:     s.Provider.Title,
			KeyURL:    s.Provider.KeyURL,
			Connected: s.Connected(),
			NoKey:     s.Provider.NoKey,
		}
		if s.Source != 0 {
			pa.Source = s.Source.String()
		}
		if s.Source == credential.SourceEnv {
			pa.EnvVar = credential.EnvVar(s.Provider.Name)
		}
		if s.Err != nil && !errors.Is(s.Err, credential.ErrNoKey) {
			pa.Err = s.Err.Error()
		}
		out[i] = pa
	}
	return out
}

// Login checks key with provider and saves it only when the provider
// accepts it. A rejected key is not saved. The result carries the credential
// store's pending one-time notice, if it has one.
func (a *Auth) Login(ctx context.Context, provider, key string) (api.LoginResult, error) {
	p, err := Find(a.providers, provider)
	if err != nil {
		return api.LoginResult{}, err
	}
	k, err := credential.NewKey(key)
	if err != nil {
		return api.LoginResult{}, err
	}
	res, err := p.Login(ctx, a.store, k)
	if err != nil {
		return api.LoginResult{}, err
	}
	notice := a.takeNotice()
	if res.Unchecked {
		notice = strings.TrimSpace(notice + " " + UncheckedText)
	}
	return api.LoginResult{Source: res.Source.String(), Shadowed: res.Shadowed, Notice: notice}, nil
}

// Logout removes provider's saved key. It returns an error matching
// credential.ErrInEnvironment when the key comes from the environment, and
// credential.ErrNoKey when nothing is saved.
func (a *Auth) Logout(provider string) error {
	p, err := Find(a.providers, provider)
	if err != nil {
		return err
	}
	return p.Logout(a.store)
}

// Notice records a one-time message from the credential store for the next
// Login result. It is the store's notice callback.
func (a *Auth) Notice(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.notice = msg
}

// takeNotice returns the pending notice and clears it.
func (a *Auth) takeNotice() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	msg := a.notice
	a.notice = ""
	return msg
}
