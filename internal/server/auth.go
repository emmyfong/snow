package server

import (
	"context"
	"errors"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// ErrNoAuth means an auth request came, but the server was started without
// ChatOptions.Auth.
var ErrNoAuth = errors.New("server: auth is not set up")

// authTimeout bounds one login or logout call, which may read the OS key
// store and the provider's API.
const authTimeout = 30 * time.Second

// Auth lists the providers and saves or removes their keys. signin.Auth
// satisfies it. Its calls may read the OS key store, so the server runs
// them off the state goroutine.
type Auth interface {
	Providers() []api.ProviderAuth
	Login(ctx context.Context, provider, key string) (api.LoginResult, error)
	Logout(provider string) error
}

// chatAuth returns the auth service, or ErrNoAuth when the server has none.
func (s *Server) chatAuth() (Auth, error) {
	o := s.opts.Chat
	if o == nil || o.Auth == nil {
		return nil, ErrNoAuth
	}
	return o.Auth, nil
}

// authList answers an AuthListRequest with the current key state. It runs on
// the reader goroutine, so the key lookup stays off the model goroutine.
func (s *Server) authList(c *client) {
	auth, err := s.chatAuth()
	if err != nil {
		s.refuse(c, api.CodeCommandFailed, err)
		return
	}
	s.reply(c, &api.AuthList{Providers: auth.Providers()})
}

// login checks a key with its provider and saves it when the provider
// accepts it. The check can take a while, so it runs in its own goroutine;
// the replies go through the state goroutine in order.
func (s *Server) login(c *client, req *api.LoginRequest) {
	auth, err := s.chatAuth()
	if err != nil {
		s.refuse(c, api.CodeCommandFailed, err)
		return
	}
	if err := req.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
		defer cancel()
		res, err := auth.Login(ctx, req.Provider, req.Key)
		if err != nil {
			s.reply(c, &api.Error{Code: api.CodeAuthFailed, Message: err.Error()})
			return
		}
		s.reply(c, &api.Login{Provider: req.Provider, LoginResult: res})
		s.reply(c, &api.AuthList{Providers: auth.Providers()})
	}()
}

// logout removes a provider's saved key. Like login, it runs off the state
// goroutine and reports a fresh provider list on success.
func (s *Server) logout(c *client, req *api.LogoutRequest) {
	auth, err := s.chatAuth()
	if err != nil {
		s.refuse(c, api.CodeCommandFailed, err)
		return
	}
	if err := req.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	go func() {
		if err := auth.Logout(req.Provider); err != nil {
			s.reply(c, &api.Error{Code: api.CodeAuthFailed, Message: err.Error()})
			return
		}
		s.reply(c, &api.Logout{Provider: req.Provider})
		s.reply(c, &api.AuthList{Providers: auth.Providers()})
	}()
}
