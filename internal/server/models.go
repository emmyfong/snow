package server

import (
	"context"
	"fmt"
	"time"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// DefaultModelTTL is how long a fetched model list stays fresh.
const DefaultModelTTL = 24 * time.Hour

// modelFetchTimeout bounds one provider's model list call.
const modelFetchTimeout = 30 * time.Second

// modelCache holds the model lists the server fetched, by provider. The
// model goroutine owns it.
type modelCache struct {
	statuses []api.ProviderModels // as last reported, in picker order
	lists    map[string][]provider.Model
	errs     map[string]error
	fetched  map[string]time.Time
	fetching map[string]bool
}

// newModelCache returns an empty cache.
func newModelCache() modelCache {
	return modelCache{
		lists:    map[string][]provider.Model{},
		errs:     map[string]error{},
		fetched:  map[string]time.Time{},
		fetching: map[string]bool{},
	}
}

// setStatuses replaces the provider list. What the picker learns from a
// fetch, such as models and errors, stays.
func (c *modelCache) setStatuses(in []api.ProviderModels) {
	c.statuses = c.statuses[:0]
	for _, p := range in {
		if p.Name == "" {
			continue
		}
		c.statuses = append(c.statuses, api.ProviderModels{Name: p.Name, Title: p.Title, Connected: p.Connected})
	}
}

// reply is the model list the client sees: the providers with the lists
// and errors fetched so far.
func (c *modelCache) reply() *api.ModelList {
	out := &api.ModelList{Providers: make([]api.ProviderModels, 0, len(c.statuses))}
	for _, status := range c.statuses {
		p := status
		for _, m := range c.lists[status.Name] {
			info := api.ModelInfo{ID: m.ID, DisplayName: m.DisplayName, ContextWindow: m.ContextWindow}
			for _, e := range m.Efforts {
				info.Efforts = append(info.Efforts, string(e))
			}
			p.Models = append(p.Models, info)
		}
		if err := c.errs[status.Name]; err != nil {
			p.Err = err.Error()
		}
		p.Refreshing = c.fetching[status.Name]
		out.Providers = append(out.Providers, p)
	}
	return out
}

// fresh reports whether name's list, or the error of its last fetch, is
// still young enough to use.
func (c *modelCache) fresh(name string, now time.Time, ttl time.Duration) bool {
	at, ok := c.fetched[name]
	return ok && now.Sub(at) < ttl
}

// done records the result of a fetch. A failed fetch keeps the list it had
// and its error stays until the next fetch, so a broken provider is not
// asked again on every request.
func (c *modelCache) done(name string, models []provider.Model, err error, at time.Time) {
	delete(c.fetching, name)
	c.fetched[name] = at
	if err != nil {
		c.errs[name] = err
		return
	}
	delete(c.errs, name)
	c.lists[name] = models
}

// modelList answers a ModelListRequest from the cache, then fetches the
// providers whose list is missing or stale. It runs on c's reader
// goroutine, so the status lookup, which may read the key store, stays off
// the model goroutine.
func (s *Server) modelList(c *client, req *api.ModelListRequest) {
	o := s.opts.Chat
	if o == nil {
		s.refuse(c, api.CodeCommandFailed, ErrNoChat)
		return
	}
	statuses := o.providerStatuses()
	s.call(func(st *state) {
		if c.dropped {
			return
		}
		c.wantsModels = true
		st.models.setStatuses(statuses)
		fetch := st.refreshModels(req.Force)
		st.enqueue(c, st.models.reply())
		for _, name := range fetch {
			go st.fetchModels(name)
		}
	})
}

// refreshModels marks the connected providers whose list is missing or
// stale, or every provider with force, and returns their names. A fetch
// that already runs is left alone.
func (st *state) refreshModels(force bool) []string {
	o := st.srv.opts.Chat
	if o == nil {
		return nil
	}
	now, ttl := o.now(), o.modelTTL()
	var fetch []string
	for _, p := range st.models.statuses {
		if !p.Connected || st.models.fetching[p.Name] {
			continue
		}
		if !force && st.models.fresh(p.Name, now, ttl) {
			continue
		}
		st.models.fetching[p.Name] = true
		fetch = append(fetch, p.Name)
	}
	return fetch
}

// fetchModels asks one provider for its models, off the model goroutine,
// and reports the result back to it.
func (st *state) fetchModels(name string) {
	o := st.srv.opts.Chat
	var models []provider.Model
	var err error
	if p, ok := o.Providers.Get(name); !ok {
		err = fmt.Errorf("no provider %s", name)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), modelFetchTimeout)
		models, err = p.Models(ctx)
		cancel()
	}
	st.srv.send(func(st *state) {
		st.models.done(name, models, err, o.now())
		st.broadcastModels()
	})
}

// broadcastModels sends the current list to every client that asked for
// it.
func (st *state) broadcastModels() {
	list := st.models.reply()
	for c := range st.clients {
		if c.wantsModels {
			st.enqueue(c, list)
		}
	}
}
